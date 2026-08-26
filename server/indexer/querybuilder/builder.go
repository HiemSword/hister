package querybuilder

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/indexer/searchschema"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
)

var (
	relativeTimeFilterPattern = regexp.MustCompile(`^(<=|>=|<|>)([0-9]+)([smhdw])$`)
	absoluteDateFilterPattern = regexp.MustCompile(`^(<=|>=|<|>)([0-9]{4}-[0-9]{2}-[0-9]{2})$`)
	ErrInvalidRegexp          = errors.New("invalid URL regexp")
)

const MaxURLRegexpLength = 4096

// BuildValidated builds a query after validating URL regular expression
// filters. Other query syntax retains Build's existing fallback behavior.
func BuildValidated(s string) (query.Query, error) {
	qt, err := Tokenize(s)
	if err == nil {
		if err := validateURLRegexps(qt); err != nil {
			return nil, err
		}
	}
	return Build(s), nil
}

func validateURLRegexps(tokens []Token) error {
	for _, token := range tokens {
		if token.Type == TokenAlternation {
			if err := validateURLRegexps(token.Parts); err != nil {
				return err
			}
			continue
		}
		pattern, ok := urlRegexpToken(token)
		if !ok {
			continue
		}
		if pattern == "" {
			return fmt.Errorf("%w: expression is empty", ErrInvalidRegexp)
		}
		if len(pattern) > MaxURLRegexpLength {
			return fmt.Errorf("%w: expression exceeds %d bytes", ErrInvalidRegexp, MaxURLRegexpLength)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("%w %q: %v", ErrInvalidRegexp, pattern, err)
		}
	}
	return nil
}

func urlRegexpToken(token Token) (string, bool) {
	if token.Type != TokenWord && token.Type != TokenQuoted {
		return "", false
	}
	value := strings.TrimPrefix(token.Value, "-")
	field, pattern, ok := fieldFilterValue(value)
	return pattern, ok && field.Kind == searchschema.FieldKindRegexp
}

func Build(s string) query.Query {
	if strings.TrimSpace(s) == "" {
		return query.NewMatchNoneQuery()
	}

	qt, err := Tokenize(s)
	if err != nil {
		return createSimpleQuery(s)
	}
	if slices.ContainsFunc(qt, isNegatedStandaloneWildcard) {
		return query.NewMatchNoneQuery()
	}
	qt = slices.DeleteFunc(qt, isMatchAllToken)
	if len(qt) == 0 {
		return query.NewMatchAllQuery()
	}

	qs := []query.Query{}
	nqs := []query.Query{}
	now := time.Now()

	for _, t := range qt {
		q, negated := getTokenQuery(t, now)
		if negated {
			nqs = append(nqs, q)
		} else {
			qs = append(qs, q)
		}
	}
	if len(qt) > 1 && !anyFieldSpecific(qt) {
		// create a full phrase query from the query string to get exact matches for the full query
		pq := createCombinedMatchQuery(s, 2)
		qs = []query.Query{
			bleve.NewDisjunctionQuery(
				bleve.NewConjunctionQuery(qs...),
				pq,
			),
		}
	}
	q := query.NewBooleanQuery(qs, nil, nqs)
	if len(qt) == 1 && !isFieldSpecific(qt[0]) {
		// prioritize base url matches if there is only one non field specific search term for easier retrieval of websites.
		uq := bleve.NewRegexpQuery(fmt.Sprintf("https?://(www\\.)?%s[^/]*/", regexp.QuoteMeta(strings.ToLower(qt[0].Value))))
		uq.SetField("url")
		uq.SetBoost(100)
		q.AddShould(uq)
	}
	return q
}

func isStandaloneWildcard(t Token) bool {
	return t.Type == TokenWord && t.Value == "*"
}

func isMatchAllToken(t Token) bool {
	if isStandaloneWildcard(t) {
		return true
	}
	return t.Type == TokenAlternation && slices.ContainsFunc(t.Parts, isStandaloneWildcard)
}

func isNegatedStandaloneWildcard(t Token) bool {
	return t.Type == TokenWord && t.Value == "-*"
}

// RemoveStandaloneWildcards returns text suitable for semantic embedding.
func RemoveStandaloneWildcards(s string) string {
	qt, err := Tokenize(s)
	if err != nil {
		return s
	}
	if slices.ContainsFunc(qt, isNegatedStandaloneWildcard) {
		return ""
	}
	qt = slices.DeleteFunc(qt, isMatchAllToken)
	parts := make([]string, len(qt))
	for i, t := range qt {
		parts[i] = t.Value
	}
	return strings.Join(parts, " ")
}

// anyFieldSpecific returns true when at least one token in qt is an explicit
// field query (e.g. url:..., domain:..., user_id:...).  When any token is
// field-specific, adding a full-string phrase query would produce semantically
// wrong results, so the caller skips the phrase-wrapping optimisation.
func anyFieldSpecific(qt []Token) bool {
	return slices.ContainsFunc(qt, isFieldSpecific)
}

// isFieldSpecific reports whether t carries an explicit field: prefix and
// therefore represents a targeted field query rather than free text.
func isFieldSpecific(t Token) bool {
	v := t.Value
	switch t.Type {
	case TokenWord, TokenQuoted:
		v = strings.TrimPrefix(v, "-")
		if strings.HasPrefix(v, "metadata.") && strings.Contains(v, ":") {
			return true
		}
		field, value, ok := fieldFilterValue(v)
		if !ok {
			return false
		}
		if field.Kind == searchschema.FieldKindTime {
			return validTimeFilter(value)
		}
		return true
	}
	return false
}

func createSimpleQuery(s string) query.Query {
	return bleve.NewQueryStringQuery(s)
}

func createCombinedMatchQuery(s string, boost float64) query.Query {
	q := bleve.NewDisjunctionQuery(
		createMatchPhraseQuery(s, boost*10),
		createMatchQuery(s, boost),
	)
	q.SetBoost(boost)
	return q
}

// Matches all terms from the query (AND semantics per field)
func createMatchQuery(s string, boost float64) query.Query {
	queries := make([]query.Query, 0)
	for _, field := range searchschema.Fields() {
		if !field.DefaultSearch {
			continue
		}
		match := bleve.NewMatchQuery(s)
		match.SetField(field.IndexField)
		match.SetBoost(field.Weight)
		match.Operator = query.MatchQueryOperatorAnd
		queries = append(queries, match)
	}
	q := bleve.NewDisjunctionQuery(queries...)
	q.SetBoost(boost)
	return q
}

// buildFieldQuery creates the appropriate Bleve query for a named field and value.
// It uses a case insensitive RegexpQuery for URL wildcards, WildcardQuery for
// other wildcard fields, TermQuery for URL and domain fields, and MatchQuery
// for all other fields.
func buildFieldQuery(field, v string) query.Query {
	definition, ok := searchschema.Field(field)
	if !ok {
		return bleve.NewQueryStringQuery(v)
	}
	if strings.Contains(v, "*") {
		if definition.NormalizeFilePath {
			q := bleve.NewRegexpQuery(caseInsensitiveWildcardRegexp(v))
			q.SetField(definition.IndexField)
			q.SetBoost(definition.Weight)
			return q
		}
		q := bleve.NewWildcardQuery(strings.ToLower(v))
		q.SetField(definition.IndexField)
		q.SetBoost(definition.Weight)
		return q
	}
	if definition.Kind == searchschema.FieldKindKeyword {
		if definition.NormalizeFilePath {
			v = normalizeFileURL(v)
		}
		q := bleve.NewTermQuery(v)
		q.SetField(definition.IndexField)
		q.SetBoost(definition.Weight)
		return q
	}
	q := bleve.NewMatchQuery(v)
	q.SetField(definition.IndexField)
	q.SetBoost(definition.Weight)
	return q
}

func caseInsensitiveWildcardRegexp(value string) string {
	var pattern strings.Builder
	pattern.WriteString("(?i)")
	for _, r := range value {
		switch r {
		case '*':
			pattern.WriteString(".*")
		case '?':
			pattern.WriteByte('.')
		default:
			pattern.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	return pattern.String()
}

// buildQuotedFieldQuery preserves phrase semantics for analyzed text fields.
// URL and other fields retain their existing query behavior.
func buildQuotedFieldQuery(field, v string) query.Query {
	definition, ok := searchschema.Field(field)
	if ok && definition.Kind == searchschema.FieldKindRegexp {
		return buildURLRegexpQuery(definition, v)
	}
	if ok && definition.Phrase {
		q := bleve.NewMatchPhraseQuery(v)
		q.SetField(definition.IndexField)
		q.SetBoost(definition.Weight)
		return q
	}
	return buildFieldQuery(field, v)
}

// buildURLRegexpQuery applies the same Go regexp MatchString semantics used by
// indexing rules. URL doc values contain the complete normalized URL.
func buildURLRegexpQuery(field searchschema.FieldDefinition, pattern string) query.Query {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return query.NewMatchNoneQuery()
	}
	return query.NewCustomFilterQueryWithFilter(
		query.NewMatchAllQuery(),
		func(match *search.DocumentMatch) bool {
			urlValue, ok := match.Fields[field.IndexField].(string)
			return ok && re.MatchString(urlValue)
		},
		[]string{field.IndexField},
		nil,
	)
}

func fieldFilterValue(value string) (searchschema.FieldDefinition, string, bool) {
	name, filterValue, ok := strings.Cut(value, ":")
	if !ok {
		return searchschema.FieldDefinition{}, "", false
	}
	field, ok := searchschema.Field(name)
	return field, filterValue, ok
}

func buildNumericRangeQuery(field searchschema.FieldDefinition, v string) (query.Query, bool) {
	parseBound := func(s string) (*float64, bool) {
		if s == "" {
			return nil, true
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, false
		}
		return &n, true
	}

	var min, max *float64
	if before, after, ok := strings.Cut(v, ".."); ok {
		var valid bool
		min, valid = parseBound(before)
		if !valid {
			return nil, false
		}
		max, valid = parseBound(after)
		if !valid {
			return nil, false
		}
	} else {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, false
		}
		min = &n
		max = &n
	}
	if min == nil && max == nil {
		return nil, false
	}
	q := bleve.NewNumericRangeInclusiveQuery(min, max, new(true), new(true))
	q.SetField(field.IndexField)
	return q, true
}

func parseRelativeTimeFilter(v string) (string, int64, bool) {
	parts := relativeTimeFilterPattern.FindStringSubmatch(strings.ToLower(v))
	if parts == nil {
		return "", 0, false
	}
	amount, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, false
	}
	var unitSeconds int64
	switch parts[3] {
	case "s":
		unitSeconds = 1
	case "m":
		unitSeconds = 60
	case "h":
		unitSeconds = 60 * 60
	case "d":
		unitSeconds = 24 * 60 * 60
	case "w":
		unitSeconds = 7 * 24 * 60 * 60
	default:
		return "", 0, false
	}
	if amount > (1<<63-1)/unitSeconds {
		return "", 0, false
	}
	return parts[1], amount * unitSeconds, true
}

func parseAbsoluteDateFilter(v string) (string, int64, bool) {
	parts := absoluteDateFilterPattern.FindStringSubmatch(v)
	if parts == nil {
		return "", 0, false
	}
	date, err := time.Parse("2006-01-02", parts[2])
	if err != nil {
		return "", 0, false
	}
	return parts[1], date.Unix(), true
}

func validTimeFilter(v string) bool {
	if _, _, ok := parseRelativeTimeFilter(v); ok {
		return true
	}
	_, _, ok := parseAbsoluteDateFilter(v)
	return ok
}

// BuildTimestampRange creates a numeric timestamp range for an indexed time
// field. It is shared by query syntax and legacy structured API parameters.
func BuildTimestampRange(field string, min, max *int64, minInclusive, maxInclusive bool) (query.Query, bool) {
	definition, ok := searchschema.Field(field)
	if !ok || definition.Kind != searchschema.FieldKindTime || (min == nil && max == nil) {
		return nil, false
	}
	var minValue, maxValue *float64
	if min != nil {
		minValue = new(float64)
		*minValue = float64(*min)
	}
	if max != nil {
		maxValue = new(float64)
		*maxValue = float64(*max)
	}
	q := bleve.NewNumericRangeInclusiveQuery(minValue, maxValue, &minInclusive, &maxInclusive)
	q.SetField(definition.IndexField)
	return q, true
}

func buildTimeQuery(fieldName, v string, now time.Time) (query.Query, bool) {
	field, ok := searchschema.Field(fieldName)
	if !ok || field.Kind != searchschema.FieldKindTime {
		return nil, false
	}
	comparison, durationSeconds, relative := parseRelativeTimeFilter(v)
	var cutoff int64
	if relative {
		cutoff = now.Unix() - durationSeconds
		switch comparison {
		case ">":
			comparison = "<"
		case ">=":
			comparison = "<="
		case "<":
			comparison = ">"
		case "<=":
			comparison = ">="
		}
	} else {
		var timestamp int64
		var ok bool
		comparison, timestamp, ok = parseAbsoluteDateFilter(v)
		if !ok {
			return nil, false
		}
		cutoff = timestamp
	}
	var min, max *int64
	minInclusive := true
	maxInclusive := true
	switch comparison {
	case ">":
		min = &cutoff
		minInclusive = false
	case ">=":
		min = &cutoff
	case "<":
		max = &cutoff
		maxInclusive = false
	case "<=":
		max = &cutoff
	default:
		return nil, false
	}
	return BuildTimestampRange(field.Name, min, max, minInclusive, maxInclusive)
}

// Matches exact phrases without stopwords
func createMatchPhraseQuery(s string, boost float64) query.Query {
	queries := make([]query.Query, 0)
	for _, field := range searchschema.Fields() {
		if !field.DefaultSearch {
			continue
		}
		match := bleve.NewMatchPhraseQuery(s)
		match.SetField(field.IndexField)
		match.SetBoost(field.Weight)
		queries = append(queries, match)
	}
	q := bleve.NewDisjunctionQuery(queries...)
	q.SetBoost(boost)
	return q
}

func buildFieldAlternationQuery(field, value string, now time.Time) (query.Query, bool) {
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return nil, false
	}
	parts, err := parseAlternationParts(value[1 : len(value)-1])
	if err != nil || len(parts) == 0 {
		return nil, false
	}
	queries := make([]query.Query, 0, len(parts))
	for _, part := range parts {
		partQuery, _ := getTokenQuery(Token{Type: TokenWord, Value: field + ":" + part.Value}, now)
		queries = append(queries, partQuery)
	}
	if len(queries) == 1 {
		return queries[0], true
	}
	return bleve.NewDisjunctionQuery(queries...), true
}

func getTokenQuery(t Token, now time.Time) (query.Query, bool) {
	negated := false
	switch t.Type {
	case TokenQuoted:
		v := t.Value
		if strings.HasPrefix(v, "-") {
			negated = true
			v = v[1:]
		}
		field, value, hasField := fieldFilterValue(v)
		if hasField && (field.Kind == searchschema.FieldKindText || field.Kind == searchschema.FieldKindKeyword || field.Kind == searchschema.FieldKindRegexp) {
			v := value
			if field.Kind == searchschema.FieldKindRegexp {
				return buildURLRegexpQuery(field, v), negated
			}
			if strings.HasPrefix(v, "-") && len(v) > 1 {
				negated = true
				v = v[1:]
			}
			return buildQuotedFieldQuery(field.Name, v), negated
		}
		return createMatchPhraseQuery(v, 1), negated
	case TokenWord:
		if strings.HasPrefix(t.Value, "-") && len(t.Value) > 1 {
			negated = true
			t.Value = t.Value[1:]
		}
		if t.Value == "*" {
			return query.NewMatchAllQuery(), negated
		}
		if strings.HasPrefix(t.Value, "metadata.") && strings.Contains(t.Value, ":") {
			metadataField, v, _ := strings.Cut(t.Value, ":")
			if q, ok := buildFieldAlternationQuery(metadataField, v, now); ok {
				return q, negated
			}
			q := bleve.NewTermQuery(v)
			q.SetField(metadataField)
			return q, negated
		}
		field, v, hasField := fieldFilterValue(t.Value)
		if hasField {
			if field.Kind != searchschema.FieldKindRegexp {
				if q, ok := buildFieldAlternationQuery(field.Name, v, now); ok {
					return q, negated
				}
			}
			switch field.Kind {
			case searchschema.FieldKindRegexp:
				return buildURLRegexpQuery(field, v), negated
			case searchschema.FieldKindEnum:
				if value, ok := searchschema.Value(field.ValueSet, v); ok {
					q := bleve.NewNumericRangeQuery(value.Min, value.Max)
					q.SetField(field.IndexField)
					return q, negated
				}
			case searchschema.FieldKindNumericRange:
				if q, ok := buildNumericRangeQuery(field, v); ok {
					return q, negated
				}
			case searchschema.FieldKindTime:
				if q, ok := buildTimeQuery(field.Name, v, now); ok {
					return q, negated
				}
			case searchschema.FieldKindInteger:
				if number, err := strconv.ParseUint(v, 10, 64); err == nil {
					value := float64(number)
					q := bleve.NewNumericRangeInclusiveQuery(&value, &value, new(true), new(true))
					q.SetField(field.IndexField)
					return q, negated
				}
			}
		}
		if hasField && (field.Kind == searchschema.FieldKindText || field.Kind == searchschema.FieldKindKeyword) {
			if strings.HasPrefix(v, "-") && len(v) > 1 {
				negated = true
				v = v[1:]
			}
			return buildFieldQuery(field.Name, v), negated
		}

		qs := []query.Query{}
		wcq := t.Value
		if !strings.HasPrefix(wcq, "*") {
			wcq = "*" + wcq
		}
		if !strings.HasSuffix(wcq, "*") {
			wcq = wcq + "*"
		}
		for _, field := range searchschema.Fields() {
			switch {
			case field.DefaultSearch:
				qs = append(qs, buildFieldQuery(field.Name, t.Value))
			case field.DefaultWildcard:
				qs = append(qs, buildFieldQuery(field.Name, wcq))
			}
		}
		return bleve.NewDisjunctionQuery(qs...), negated

	case TokenAlternation:
		qs := []query.Query{}
		for _, p := range t.Parts {
			r, _ := getTokenQuery(p, now)
			qs = append(qs, r)
		}
		return bleve.NewDisjunctionQuery(qs...), negated
	}
	return bleve.NewQueryStringQuery(t.Value), negated
}

func normalizeFileURL(v string) string {
	if strings.HasPrefix(v, "*") {
		return v
	}
	if strings.Contains(v, "://") {
		return v
	}
	if abs, err := filepath.Abs(v); err == nil {
		v = abs
	}
	return files.PathToFileURL(v)
}

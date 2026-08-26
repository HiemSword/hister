package querybuilder

import "testing"

func Test_tokenize_arabic_word(t *testing.T) {
	tokens, err := Tokenize("سلام")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenWord {
		t.Fatalf("expected TokenWord, got %v", tokens[0].Type)
	}
	if tokens[0].Value != "سلام" {
		t.Fatalf("expected token value %q, got %q", "سلام", tokens[0].Value)
	}
}

func Test_tokenize_english_word(t *testing.T) {
	tokens, err := Tokenize("hello")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenWord {
		t.Fatalf("expected TokenWord, got %v", tokens[0].Type)
	}
	if tokens[0].Value != "hello" {
		t.Fatalf("expected token value %q, got %q", "hello", tokens[0].Value)
	}
}

func Test_tokenize_english_quoted_phrase(t *testing.T) {
	tokens, err := Tokenize(`"hello world"`)
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenQuoted {
		t.Fatalf("expected TokenQuoted, got %v", tokens[0].Type)
	}
	if tokens[0].Value != "hello world" {
		t.Fatalf("expected token value %q, got %q", "hello world", tokens[0].Value)
	}
}

func Test_tokenize_english_alternation(t *testing.T) {
	tokens, err := Tokenize("(hello|world)")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenAlternation {
		t.Fatalf("expected TokenAlternation, got %v", tokens[0].Type)
	}
	if len(tokens[0].Parts) != 2 {
		t.Fatalf("expected 2 alternation parts, got %d", len(tokens[0].Parts))
	}
	if tokens[0].Parts[0].Value != "hello" {
		t.Fatalf("expected first alternation part %q, got %q", "hello", tokens[0].Parts[0].Value)
	}
	if tokens[0].Parts[1].Value != "world" {
		t.Fatalf("expected second alternation part %q, got %q", "world", tokens[0].Parts[1].Value)
	}
}

func Test_tokenize_alternation_keeps_last_arabic_part(t *testing.T) {
	tokens, err := Tokenize("(hello|مرحبا)")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenAlternation {
		t.Fatalf("expected TokenAlternation, got %v", tokens[0].Type)
	}
	if len(tokens[0].Parts) != 2 {
		t.Fatalf("expected 2 alternation parts, got %d", len(tokens[0].Parts))
	}
	if tokens[0].Parts[1].Value != "مرحبا" {
		t.Fatalf("expected last alternation part %q, got %q", "مرحبا", tokens[0].Parts[1].Value)
	}
}

func Test_tokenize_empty_input(t *testing.T) {
	tokens, err := Tokenize("")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}

func Test_tokenize_whitespace_only(t *testing.T) {
	tokens, err := Tokenize("   ")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}

func Test_tokenize_multiple_words(t *testing.T) {
	tokens, err := Tokenize("foo bar baz")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	for i, want := range []string{"foo", "bar", "baz"} {
		if tokens[i].Type != TokenWord {
			t.Fatalf("token[%d]: expected TokenWord, got %v", i, tokens[i].Type)
		}
		if tokens[i].Value != want {
			t.Fatalf("token[%d]: expected %q, got %q", i, want, tokens[i].Value)
		}
	}
}

func Test_tokenize_negated_word(t *testing.T) {
	tokens, err := Tokenize("-hello")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenWord {
		t.Fatalf("expected TokenWord, got %v", tokens[0].Type)
	}
	if tokens[0].Value != "-hello" {
		t.Fatalf("expected %q, got %q", "-hello", tokens[0].Value)
	}
}

func Test_tokenize_single_dash_is_word(t *testing.T) {
	tokens, err := Tokenize("-")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenWord {
		t.Fatalf("expected TokenWord, got %v", tokens[0].Type)
	}
	if tokens[0].Value != "-" {
		t.Fatalf("expected %q, got %q", "-", tokens[0].Value)
	}
}

func Test_tokenize_field_prefix(t *testing.T) {
	tokens, err := Tokenize("title:golang")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenWord {
		t.Fatalf("expected TokenWord, got %v", tokens[0].Type)
	}
	if tokens[0].Value != "title:golang" {
		t.Fatalf("expected %q, got %q", "title:golang", tokens[0].Value)
	}
}

func Test_tokenize_unquoted_url_regexp_preserves_backslashes(t *testing.T) {
	input := `url_re:^https?://example\.com/(private|login)$`
	tokens, err := Tokenize(input)
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Type != TokenWord || tokens[0].Value != input {
		t.Fatalf("Tokenize(%q) = %#v", input, tokens)
	}
}

func Test_tokenize_quoted_url_regexp_preserves_backslashes(t *testing.T) {
	input := `url_re:"^file:///.*/My Documents/.*\.pdf$"`
	tokens, err := Tokenize(input)
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	want := `url_re:^file:///.*/My Documents/.*\.pdf$`
	if len(tokens) != 1 || tokens[0].Type != TokenQuoted || tokens[0].Value != want {
		t.Fatalf("Tokenize(%q) = %#v, want value %q", input, tokens, want)
	}
}

func Test_tokenize_mixed_tokens(t *testing.T) {
	tokens, err := Tokenize(`golang "web framework" (gin|echo)`)
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Type != TokenWord {
		t.Fatalf("token[0]: expected TokenWord, got %v", tokens[0].Type)
	}
	if tokens[1].Type != TokenQuoted {
		t.Fatalf("token[1]: expected TokenQuoted, got %v", tokens[1].Type)
	}
	if tokens[2].Type != TokenAlternation {
		t.Fatalf("token[2]: expected TokenAlternation, got %v", tokens[2].Type)
	}
}

func Test_tokenize_escaped_quote_in_quoted(t *testing.T) {
	tokens, err := Tokenize(`"say \"hello\""`)
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenQuoted {
		t.Fatalf("expected TokenQuoted, got %v", tokens[0].Type)
	}
	if tokens[0].Value != `say "hello"` {
		t.Fatalf("expected %q, got %q", `say "hello"`, tokens[0].Value)
	}
}

func Test_tokenize_unclosed_alternation_returns_error(t *testing.T) {
	_, err := Tokenize("(foo|bar")
	if err == nil {
		t.Fatal("expected error for unclosed alternation, got nil")
	}
}

func Test_tokenize_alternation_whitespace_trimmed(t *testing.T) {
	tokens, err := Tokenize("( foo | bar )")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Type != TokenAlternation {
		t.Fatalf("expected 1 alternation token, got %v", tokens)
	}
	if len(tokens[0].Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(tokens[0].Parts))
	}
	if tokens[0].Parts[0].Value != "foo" {
		t.Fatalf("expected part[0] %q, got %q", "foo", tokens[0].Parts[0].Value)
	}
	if tokens[0].Parts[1].Value != "bar" {
		t.Fatalf("expected part[1] %q, got %q", "bar", tokens[0].Parts[1].Value)
	}
}

func Test_tokenize_alternation_single_part(t *testing.T) {
	tokens, err := Tokenize("(only)")
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Type != TokenAlternation {
		t.Fatalf("expected 1 alternation token, got %v", tokens)
	}
	if len(tokens[0].Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(tokens[0].Parts))
	}
}

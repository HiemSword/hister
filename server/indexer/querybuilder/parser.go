package querybuilder

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenType int

const (
	TokenWord TokenType = iota
	TokenQuoted
	TokenAlternation
	TokenEOF
)

type Token struct {
	Type  TokenType
	Value string
	Parts []Token
	start int
	end   int
}

type Lexer struct {
	input []rune
	pos   int
	char  rune
}

func New(input string) *Lexer {
	l := &Lexer{input: []rune(input)}
	l.readChar()
	return l
}

func (t Token) String() string {
	var tn string
	switch t.Type {
	case TokenWord:
		tn = "Word"
	case TokenQuoted:
		tn = "Quoted"
	case TokenAlternation:
		tn = "Alternation"
	case TokenEOF:
		tn = "EOF"
	default:
		tn = "Unknown"
	}
	return fmt.Sprintf("[%s: %s (%q)]", tn, t.Value, t.Parts)
}

func (l *Lexer) readChar() {
	if l.pos >= len(l.input) {
		l.char = 0
	} else {
		l.char = l.input[l.pos]
	}
	l.pos++
}

func (l *Lexer) peekChar() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(l.char) {
		l.readChar()
	}
}

func (l *Lexer) NextToken() (Token, error) {
	l.skipWhitespace()

	start := l.offset()
	var (
		token Token
		err   error
	)
	switch l.char {
	case 0:
		token = Token{Type: TokenEOF}
	case '"':
		token, err = l.readQuoted()
	case '(':
		token, err = l.readAlternation()
	default:
		token, err = l.readWord()
	}
	if err != nil {
		return Token{}, err
	}
	token.start = start
	token.end = l.offset()
	return token, nil
}

func (l *Lexer) offset() int {
	offset := l.pos - 1
	if offset < 0 {
		return 0
	}
	if offset > len(l.input) {
		return len(l.input)
	}
	return offset
}

func (l *Lexer) readQuoted() (Token, error) {
	l.readChar()
	var builder strings.Builder

	for l.char != '"' && l.char != 0 {
		if l.char == '\\' && l.peekChar() == '"' {
			l.readChar()
			builder.WriteRune('"')
			l.readChar()
			continue
		}
		builder.WriteRune(l.char)
		l.readChar()
	}

	if l.char == 0 {
		return Token{Type: TokenQuoted, Value: builder.String()}, nil
	}

	if l.char != '"' {
		return Token{}, fmt.Errorf("unclosed quoted string")
	}

	l.readChar()
	return Token{Type: TokenQuoted, Value: builder.String()}, nil
}

func (l *Lexer) readAlternation() (Token, error) {
	l.readChar()
	var builder strings.Builder
	depth := 1

	for depth > 0 && l.char != 0 {
		if l.char == '(' {
			depth++
		} else if l.char == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
		builder.WriteRune(l.char)
		l.readChar()
	}

	if l.char != ')' {
		return Token{}, fmt.Errorf("unclosed alternation string")
	}

	l.readChar()
	value := builder.String()

	parts, err := parseAlternationParts(value)
	if err != nil {
		return Token{}, err
	}

	return Token{Type: TokenAlternation, Value: value, Parts: parts}, nil
}

func parseAlternationParts(value string) ([]Token, error) {
	parts := []Token{}
	var sb strings.Builder
	depth := 0

	for _, ch := range value {
		if ch == '(' {
			depth++
			sb.WriteRune(ch)
		} else if ch == ')' {
			depth--
			sb.WriteRune(ch)
		} else if ch == '|' && depth == 0 {
			optStr := strings.TrimSpace(sb.String())
			if optStr != "" {
				token := Token{Type: TokenWord, Value: optStr}
				parts = append(parts, token)
			}
			sb.Reset()
		} else {
			sb.WriteRune(ch)
		}
	}

	optStr := strings.TrimSpace(sb.String())
	if optStr != "" {
		token := Token{Type: TokenWord, Value: optStr}
		parts = append(parts, token)
	}

	return parts, nil
}

func (l *Lexer) readWord() (Token, error) {
	var builder strings.Builder
	tt := TokenWord
	remaining := string(l.input[l.pos-1:])
	preserveBackslashes := strings.HasPrefix(remaining, "url_re:") || strings.HasPrefix(remaining, "-url_re:")

	quote := false
	escaped := false
	for (quote || !unicode.IsSpace(l.char)) && l.char != 0 {
		skip := false
		if l.char == '"' && !escaped {
			tt = TokenQuoted
			skip = true
			quote = !quote
		}
		if l.char == '\\' && !escaped {
			escaped = true
			if preserveBackslashes {
				builder.WriteRune(l.char)
			}
		} else {
			escaped = false
			if !skip {
				builder.WriteRune(l.char)
			}
		}
		l.readChar()
	}

	return Token{Type: tt, Value: builder.String()}, nil
}

func Tokenize(input string) ([]Token, error) {
	lexer := New(input)
	tokens := []Token{}

	for {
		token, err := lexer.NextToken()
		if err != nil {
			return nil, err
		}
		if token.Type == TokenEOF {
			break
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

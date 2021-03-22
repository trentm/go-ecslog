package kqlog

import (
	"testing"
)

type lexTestCase struct {
	name   string
	input  string
	tokens []token
}

func mkToken(typ tokType, text string) token {
	return token{
		typ: typ,
		val: text,
	}
}

var (
	tokEOF        = mkToken(tokTypeEOF, "")
	tokColon      = mkToken(tokTypeColon, ":")
	tokAnd        = mkToken(tokTypeAnd, "and")
	tokOr         = mkToken(tokTypeOr, "or")
	tokNot        = mkToken(tokTypeNot, "not")
	tokOpenParen  = mkToken(tokTypeOpenParen, "(")
	tokCloseParen = mkToken(tokTypeCloseParen, ")")
	tokGt         = mkToken(tokTypeGt, ">")
	tokGte        = mkToken(tokTypeGte, ">=")
	tokLt         = mkToken(tokTypeLt, "<")
	tokLte        = mkToken(tokTypeLte, "<=")
)

var lexTestCases = []lexTestCase{
	{"empty", "", []token{tokEOF}},
	{"spaces ignored", " \t\n", []token{tokEOF}},
	{"value expression", "foo", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokEOF,
	}},
	{"value expression with spaces", " foo \t\n ", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokEOF,
	}},
	{"value expression with two values", "foo bar", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		mkToken(tokTypeUnquotedLiteral, "bar"),
		tokEOF,
	}},
	{"and binary operator", "foo and bar", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokAnd,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		tokEOF,
	}},
	{"or binary operator", "foo or bar", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokOr,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		tokEOF,
	}},
	{"not binary operator", "not foo", []token{
		tokNot,
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokEOF,
	}},
	{"not binary operator on parenthesized group", "not (foo or bar)", []token{
		tokNot,
		tokOpenParen,
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokOr,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		tokCloseParen,
		tokEOF,
	}},
	{"mixed 'or' and 'and' (precendence not relevant for lexing)", "foo or bar and baz or qux", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokOr,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		tokAnd,
		mkToken(tokTypeUnquotedLiteral, "baz"),
		tokOr,
		mkToken(tokTypeUnquotedLiteral, "qux"),
		tokEOF,
	}},
	{"field value expression", "foo:bar", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		tokEOF,
	}},
	{"field value expression, multiple values", "foo:bar baz", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		mkToken(tokTypeUnquotedLiteral, "baz"),
		tokEOF,
	}},

	{"range operators 1", "bytes > 1000 and bytes < 8000", []token{
		mkToken(tokTypeUnquotedLiteral, "bytes"),
		tokGt,
		mkToken(tokTypeUnquotedLiteral, "1000"),
		tokAnd,
		mkToken(tokTypeUnquotedLiteral, "bytes"),
		tokLt,
		mkToken(tokTypeUnquotedLiteral, "8000"),
		tokEOF,
	}},
	{"range operators 2", "bytes >= 1000 and bytes <= 8000", []token{
		mkToken(tokTypeUnquotedLiteral, "bytes"),
		tokGte,
		mkToken(tokTypeUnquotedLiteral, "1000"),
		tokAnd,
		mkToken(tokTypeUnquotedLiteral, "bytes"),
		tokLte,
		mkToken(tokTypeUnquotedLiteral, "8000"),
		tokEOF,
	}},
	{"date range 1", `created_at >= "2021" and created_at < "2021-03-21T16:51:43.694Z"`, []token{
		mkToken(tokTypeUnquotedLiteral, "created_at"),
		tokGte,
		mkToken(tokTypeQuotedLiteral, `"2021"`),
		tokAnd,
		mkToken(tokTypeUnquotedLiteral, "created_at"),
		tokLt,
		mkToken(tokTypeQuotedLiteral, `"2021-03-21T16:51:43.694Z"`),
		tokEOF,
	}},

	{"wildcard in field name", "machine*:osx", []token{
		mkToken(tokTypeUnquotedLiteral, "machine*"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "osx"),
		tokEOF,
	}},
	{"wildcard in value", "foo:ba*", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "ba*"),
		tokEOF,
	}},
	{"wildcard in value, exists query", "foo:*", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "*"),
		tokEOF,
	}},

	{"do not support nested queries", "nestedField:{ childOfNested: foo }", []token{
		mkToken(tokTypeUnquotedLiteral, "nestedField"),
		tokColon,
		mkToken(tokTypeError, "do not support KQL nested field queries: '{'"),
	}},

	// Escapes
	{"escapes: colon", "foo:bar\\:", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "bar\\:"),
		tokEOF,
	}},
	{"escapes: escaped operator and", "foo:bar \\and", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		mkToken(tokTypeUnquotedLiteral, "\\and"),
		tokEOF,
	}},
	{"escapes: escaped operator or", "foo:bar \\or", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		mkToken(tokTypeUnquotedLiteral, "\\or"),
		tokEOF,
	}},
	{"escapes: escaped operator not", "foo:bar \\not", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeUnquotedLiteral, "bar"),
		mkToken(tokTypeUnquotedLiteral, "\\not"),
		tokEOF,
	}},
	{"escapes: invalid end in backslash", "foo:bar\\", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeError, "unterminated character escape"),
	}},

	// Quoted literals
	{"quoted literal term", `foo:"bar baz"`, []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeQuotedLiteral, `"bar baz"`),
		tokEOF,
	}},
	{"quoted literal term surrounding space", "\t\n \"bar\tbaz\" ", []token{
		mkToken(tokTypeQuotedLiteral, "\"bar\tbaz\""),
		tokEOF,
	}},
	{"empty quoted literal", `foo:""`, []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeQuotedLiteral, `""`),
		tokEOF,
	}},
	{"quoted literal escaped double-quote", `foo:"bar\"bling"`, []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokColon,
		mkToken(tokTypeQuotedLiteral, `"bar\"bling"`),
		tokEOF,
	}},

	// Error cases
	// Ideally there is a test case for each `l.errorf()` in lex.go.
	{"error case: unclosed open parenthesis", "(foo", []token{
		tokOpenParen,
		mkToken(tokTypeUnquotedLiteral, "foo"),
		mkToken(tokTypeError, "unclosed open parenthesis"),
	}},
	{"error case: unclosed open parentheses", "(((foo", []token{
		tokOpenParen,
		tokOpenParen,
		tokOpenParen,
		mkToken(tokTypeUnquotedLiteral, "foo"),
		mkToken(tokTypeError, "unclosed open parentheses (3)"),
	}},
	{"error case: unmatched close parenthesis", "foo)", []token{
		mkToken(tokTypeUnquotedLiteral, "foo"),
		tokCloseParen,
		mkToken(tokTypeError, "unmatched close parenthesis"),
	}},
	{"error case: invalid nul char at start of token", "\u0000foo", []token{
		mkToken(tokTypeError, "unrecognized character: U+0000"),
	}},
	{"error case: unterminated escape", "foo\\", []token{
		mkToken(tokTypeError, "unterminated character escape"),
	}},
	{"error case: unterminated escape in quoted literal", "\"foo\\", []token{
		mkToken(tokTypeError, "unterminated character escape"),
	}},
	{"error case: unterminated quoted literal", "\"foo", []token{
		mkToken(tokTypeError, "unterminated quoted literal"),
	}},
}

// collectTokens gathers the emitted items into a slice.
func collectTokens(tc *lexTestCase) (tokens []token) {
	l := lex(tc.name, tc.input)
	for {
		tok := l.nextToken()
		tokens = append(tokens, tok)
		if tok.typ == tokTypeEOF || tok.typ == tokTypeError {
			break
		}
	}
	return
}

func equalTokens(i1, i2 []token) bool {
	if len(i1) != len(i2) {
		return false
	}
	for k := range i1 {
		if i1[k].typ != i2[k].typ {
			return false
		}
		if i1[k].val != i2[k].val {
			return false
		}
	}
	return true
}

func TestLex(t *testing.T) {
	for _, tc := range lexTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("-- lex test case %q\n", tc.name)
			t.Logf("  input: %#v\n", tc.input)
			tokens := collectTokens(&tc)
			t.Logf("  tokens:\n\t%#v\n\t%v\n", tokens, tokens)
			if !equalTokens(tokens, tc.tokens) {
				t.Errorf("%s: got\n\t%+v\nexpected\n\t%v\ninput\n\t%s",
					tc.name, tokens, tc.tokens, tc.input)
			}
		})
	}
}

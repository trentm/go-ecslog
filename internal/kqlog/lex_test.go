package kqlog

import (
	"fmt"
	"testing"
)

// debugf prints debug output for this test run. Uncomment the fmt.Printf to
// see it.
func debugf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

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
		mkToken(tokTypeError, "do not support KQL nest field queries: '{'"),
	}},

	// TODO: quoted literals
	// {"field value expression, quoted literal value", `foo:"bar baz"`, []token{
	// 	mkToken(tokTypeUnquotedLiteral, "foo"),
	// 	tokColon,
	// 	mkToken(tokTypeQuotedLiteral, `"bar baz"`),
	// 	tokEOF,
	// }},

	// TODO: add "escaping" test cases from kibana/src/plugins/data/common/es_query/kuery/ast/ast.test.ts
	// TODO: test each of the errorf cases in lex.go
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

func equalTokens(i1, i2 []token, checkPos bool) bool {
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
		if checkPos && i1[k].pos != i2[k].pos {
			return false
		}
	}
	return true
}

func TestLex(t *testing.T) {
	for _, tc := range lexTestCases {
		debugf("-- lex test case %q\n", tc.name)
		debugf("  input: %#v\n", tc.input)
		tokens := collectTokens(&tc)
		debugf("  tokens:\n\t%#v\n\t%v\n", tokens, tokens)
		if !equalTokens(tokens, tc.tokens, false) {
			t.Errorf("%s: got\n\t%+v\nexpected\n\t%v\ninput\n\t%s",
				tc.name, tokens, tc.tokens, tc.input)
		}
	}
}

// TODO: lexPosTests from go/src/text/template/parse/lex_test.go?
// TODO: TestShutdown from go/src/text/template/parse/lex_test.go?

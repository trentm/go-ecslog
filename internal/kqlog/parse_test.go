package kqlog

import (
	"reflect"
	"strings"
	"testing"
)

type parseTestCase struct {
	name      string
	input     string
	filter    *Filter
	errSubstr string // expected substring of error from parsing
}

var parseTestCases = []parseTestCase{
	{
		"empty",
		"",
		&Filter{steps: []rpnStep(nil)},
		"",
	},
	{
		"spaces ignored",
		" \t\n",
		&Filter{steps: []rpnStep(nil)},
		"",
	},

	{
		"default fields terms query",
		"foo",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("foo")}},
		}},
		"",
	},
	{
		"default fields terms query, multiple terms, quoted",
		"foo bar \"eggs spam\"",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []term{
				newTerm("foo"),
				newTerm("bar"),
				newQuotedTerm(`"eggs spam"`),
			}},
		}},
		"",
	},
	{
		"default fields terms query, quoted",
		"\"eggs spam\"",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []term{
				newQuotedTerm(`"eggs spam"`),
			}},
		}},
		"",
	},

	{
		"terms query",
		"foo:bar",
		&Filter{steps: []rpnStep{
			&rpnTermsQuery{field: "foo", terms: []term{newTerm("bar")}},
		}},
		"",
	},
	{
		"terms query: multiple terms",
		"foo:bar baz",
		&Filter{steps: []rpnStep{
			&rpnTermsQuery{field: "foo", terms: []term{newTerm("bar"), newTerm("baz")}},
		}},
		"",
	},
	{
		"terms query: quoted terms",
		`foo:"bar baz" bling\"`,
		&Filter{steps: []rpnStep{
			&rpnTermsQuery{field: "foo", terms: []term{
				newQuotedTerm(`"bar baz"`),
				newTerm(`bling"`),
			}},
		}},
		"",
	},

	// Match all terms queries
	{
		"match all terms query",
		"foo:(bar and baz)",
		&Filter{steps: []rpnStep{
			&rpnMatchAllTermsQuery{field: "foo", terms: []term{newTerm("bar"), newTerm("baz")}},
		}},
		"",
	},
	{
		"match all terms query, quoted term",
		`foo:(bar and "baz blah")`,
		&Filter{steps: []rpnStep{
			&rpnMatchAllTermsQuery{field: "foo", terms: []term{
				newTerm("bar"),
				newQuotedTerm(`"baz blah"`),
			}},
		}},
		"",
	},

	// Range queries
	{
		"range query",
		"foo > 42",
		&Filter{steps: []rpnStep{
			&rpnGtRangeQuery{field: "foo", term: newTerm("42")},
		}},
		"",
	},
	{
		"date range query, quoted",
		"dob <= \"1970-01-01T\"",
		&Filter{steps: []rpnStep{
			&rpnLteRangeQuery{field: "dob", term: newQuotedTerm(`"1970-01-01T"`)},
		}},
		"",
	},
	{
		"range query, escaped keyword value",
		"foo > \\and",
		&Filter{steps: []rpnStep{
			&rpnGtRangeQuery{field: "foo", term: newTerm("and")},
		}},
		"",
	},

	// Wildcard term queries
	{
		"wildcard term 1",
		"foo:ba*",
		&Filter{steps: []rpnStep{
			&rpnTermsQuery{field: "foo", terms: []term{newTerm("ba*")}},
		}},
		"",
	},
	{
		"wildcard term 2",
		"foo:b\\ta*",
		&Filter{steps: []rpnStep{
			&rpnTermsQuery{field: "foo", terms: []term{newTerm("b\ta*")}},
		}},
		"",
	},
	{
		"wildcard term with escaped and unescaped asterisks",
		"foo:f\\*\\*k*",
		&Filter{steps: []rpnStep{
			&rpnTermsQuery{field: "foo", terms: []term{newTerm("f\\*\\*k*")}},
		}},
		"",
	},
	{
		"match all terms query with wildcard",
		"foo:(bar and *az)",
		&Filter{steps: []rpnStep{
			&rpnMatchAllTermsQuery{field: "foo", terms: []term{newTerm("bar"), newTerm("*az")}},
		}},
		"",
	},

	{
		"operator precedence: and/or",
		"a and b or c and d",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("a")}},
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("b")}},
			&rpnAnd{},
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("c")}},
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("d")}},
			&rpnAnd{},
			&rpnOr{},
		}},
		"",
	},
	{
		"operator precedence: and/not/parens",
		"a and not (b or c) and d",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("a")}},
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("b")}},
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("c")}},
			&rpnOr{},
			&rpnNot{},
			&rpnAnd{},
			&rpnDefaultFieldsTermsQuery{terms: []term{newTerm("d")}},
			&rpnAnd{},
		}},
		"",
	},

	// Error cases
	// Ideally we have a test case for each `p.errorfAt()` in parse.go.
	{
		"error case: too early EOF",
		"foo:",
		nil,
		"expected a literal or '('; got EOF",
	},
	{
		"error case: too early EOF, shows context",
		"foo:",
		nil,
		"    foo:\n    ....^",
	},
	{
		"error case: lexer error token",
		"nestedField:{ childOfNested: foo }",
		nil,
		"do not support KQL nested field queries",
	},
	{
		"error case: no wildcard in range query term",
		"foo < bar*",
		nil,
		"cannot have a wildcard in range query token",
	},
	{
		"error case: range query syntax",
		"foo < : bar",
		nil,
		"expected a literal after '<'; got :",
	},
	{
		"error case: parenthesized terms 1",
		"foo:(foo and)",
		nil,
		"expected literal, got )",
	},
	{
		"error case: parenthesized terms 2",
		"foo:(bar :",
		nil,
		"expected ')', 'or', or 'and'; got :",
	},
	{
		"error case: parenthesized terms 3",
		"foo:(bar and baz or bling)",
		nil,
		"cannot mix 'and' and 'or' in parenthesized value group",
	},
	{
		"error case: parenthesized terms 4",
		"foo: : bar",
		nil,
		"expected a literal or '('; got :",
	},
	{
		"error case: incomplete 1",
		"(foo and)",
		nil,
		"expecting a literal, 'not', or '('; got )",
	},
	{
		"error case: incomplete 2",
		"foo and",
		nil,
		"incomplete boolean operator",
	},
	{
		"error case: unmatched close parenthesis",
		"(foo))",
		nil,
		"unmatched close parenthesis",
	},
}

func equalErrSubstr(err error, errSubstr string) bool {
	if err == nil {
		return errSubstr == ""
	} else if errSubstr == "" {
		return false
	}
	return strings.Contains(err.Error(), errSubstr)
}

func TestParse(t *testing.T) {
	for _, tc := range parseTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("-- parse test case %q\n", tc.name)
			t.Logf("  input: %#v\n", tc.input)
			// nil for logLevelLess arg because it isn't relevant for parsing.
			p := newParser(tc.input, nil)
			f, err := p.parse()
			if err != nil {
				t.Logf("  err: %q\n", err)
			}
			if f != nil {
				t.Logf("  filter steps:\n")
				for _, s := range f.steps {
					t.Logf("    %s\n", s)
				}
			}
			if !equalErrSubstr(err, tc.errSubstr) {
				t.Errorf(
					"%s:\n"+
						"input:\n"+
						"\t%s\n"+
						"got error:\n"+
						"\t%+v\n"+
						"expected error with this substring:\n"+
						"\t%q\n",
					tc.name, tc.input, err, tc.errSubstr)
			}
			if !reflect.DeepEqual(f, tc.filter) {
				t.Errorf(
					"%s:\n"+
						"input:\n"+
						"\t%s\n"+
						"got filter:\n"+
						"\t%v\n"+
						"expected filter:\n"+
						"\t%v\n",
					tc.name, tc.input, f, tc.filter)
			}
		})
	}
}

// TODO: lexPosTests from go/src/text/template/parse/lex_test.go?
// TODO: TestShutdown from go/src/text/template/parse/lex_test.go?

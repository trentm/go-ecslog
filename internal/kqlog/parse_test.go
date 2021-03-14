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
		"terms query with no field name",
		"foo",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []string{"foo"}},
		}},
		"",
	},
	{
		"terms query with no field name, multiple terms",
		"foo bar baz",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []string{"foo", "bar", "baz"}},
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

	// Match all terms queries
	{
		"match all terms query",
		"foo:(bar and baz)",
		&Filter{steps: []rpnStep{
			&rpnMatchAllTermsQuery{field: "foo", terms: []term{newTerm("bar"), newTerm("baz")}},
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
			&rpnDefaultFieldsTermsQuery{terms: []string{"a"}},
			&rpnDefaultFieldsTermsQuery{terms: []string{"b"}},
			&rpnAnd{},
			&rpnDefaultFieldsTermsQuery{terms: []string{"c"}},
			&rpnDefaultFieldsTermsQuery{terms: []string{"d"}},
			&rpnAnd{},
			&rpnOr{},
		}},
		"",
	},
	{
		"operator precedence: and/not/parens",
		"a and not (b or c) and d",
		&Filter{steps: []rpnStep{
			&rpnDefaultFieldsTermsQuery{terms: []string{"a"}},
			&rpnDefaultFieldsTermsQuery{terms: []string{"b"}},
			&rpnDefaultFieldsTermsQuery{terms: []string{"c"}},
			&rpnOr{},
			&rpnNot{},
			&rpnAnd{},
			&rpnDefaultFieldsTermsQuery{terms: []string{"d"}},
			&rpnAnd{},
		}},
		"",
	},

	// TODO: lots of tests to fill out here
	// TODO: add one test for each `p.errorfAt()` case
}

func equalErrSubstr(err error, errSubstr string) bool {
	if err == nil {
		return errSubstr == ""
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
			t.Logf("  filter steps:\n")
			for _, s := range f.steps {
				t.Logf("\t%s\n", s)
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
			if !equalErrSubstr(err, tc.errSubstr) {
				t.Errorf(
					"%s:\n"+
						"input:\n"+
						"\t%s\n"+
						"got error:\n"+
						"\t%+v\n"+
						"expected error with this substring:\n"+
						"\t%v\n",
					tc.name, tc.input, err, tc.errSubstr)
			}
		})
	}
}

// TODO: lexPosTests from go/src/text/template/parse/lex_test.go?
// TODO: TestShutdown from go/src/text/template/parse/lex_test.go?

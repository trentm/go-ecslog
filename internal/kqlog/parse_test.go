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
			&rpnTermsQuery{field: "foo", terms: []string{"bar"}},
		}},
		"",
	},
	{
		"terms query, multiple terms",
		"foo:bar baz",
		&Filter{steps: []rpnStep{
			&rpnTermsQuery{field: "foo", terms: []string{"bar", "baz"}},
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
		debugf("-- parse test case %q\n", tc.name)
		debugf("  input: %#v\n", tc.input)
		p := newParser(tc.input)
		f, err := p.parse()
		debugf("  filter steps:\n")
		for _, s := range f.steps {
			debugf("\t%s\n", s)
		}
		if !reflect.DeepEqual(f, tc.filter) {
			t.Errorf("%s:\ninput:\n\t%s\ngot filter:\n\t%+v\nexpected filter:\n\t%+v\n",
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
	}
}

// // TODO: lexPosTests from go/src/text/template/parse/lex_test.go?
// // TODO: TestShutdown from go/src/text/template/parse/lex_test.go?

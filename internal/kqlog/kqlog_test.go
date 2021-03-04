package kqlog

import (
	"testing"

	"github.com/valyala/fastjson"
)

type matchTestCase struct {
	name  string
	kql   string
	rec   *fastjson.Value
	match bool
}

var matchTestCases = []matchTestCase{
	{
		"empty KQL matches all",
		"",
		fastjson.MustParse(`{"foo": "bar"}`),
		true,
	},
	{
		"exists query",
		"foo:*",
		fastjson.MustParse(`{"foo": "bar"}`),
		true,
	},
	{
		"exists query: false",
		"baz:*",
		fastjson.MustParse(`{"foo": "bar"}`),
		false,
	},
	{
		"terms query",
		"foo:bar",
		fastjson.MustParse(`{"foo": "bar"}`),
		true,
	},
	{
		"terms query: false",
		"foo :baz",
		fastjson.MustParse(`{"foo": "bar"}`),
		false,
	},
	{
		"terms query: multiple values",
		"foo: bar baz",
		fastjson.MustParse(`{"foo": "baz"}`),
		true,
	},
	// TODO: more tests
}

func TestMatch(t *testing.T) {
	for _, tc := range matchTestCases {
		debugf("-- match test case %q\n", tc.name)
		debugf("  kql: %s\n", tc.kql)
		debugf("  rec: %s\n", tc.rec)
		filter, err := NewFilter(tc.kql)
		if err != nil {
			t.Errorf("%s: NewFilter(kql) error: %s\nkql:\n\t%s\n",
				tc.name, err, tc.kql)
			continue
		}
		debugf("  filter: %s\n", filter)
		match := filter.Match(tc.rec)
		debugf("  match: %v\n", match)
		if match != tc.match {
			t.Errorf(
				"%s:\n"+
					"kql:\n"+
					"\t%s\n"+
					"rec:\n"+
					"\t%s\n"+
					"got:\n"+
					"\t%v\n"+
					"expected:\n"+
					"\t%v\n",
				tc.name, tc.kql, tc.rec, match, tc.match)
		}
	}
}

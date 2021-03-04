package kqlog

import (
	"testing"

	"github.com/valyala/fastjson"
)

type lookupTestCase struct {
	name   string
	obj    *fastjson.Value
	lookup []string
	val    *fastjson.Value
}

var lookupTestCases = []lookupTestCase{
	{
		"empty object",
		fastjson.MustParse(`{}`),
		[]string{"foo"},
		nil,
	},
	{
		"nil obj",
		nil,
		[]string{"foo"},
		nil,
	},
	{
		"single name",
		fastjson.MustParse(`{"foo": "bar"}`),
		[]string{"foo"},
		fastjson.MustParse(`"bar"`),
	},
	{
		"two names: nested",
		fastjson.MustParse(`{"foo": {"bar": "baz"}}`),
		[]string{"foo", "bar"},
		fastjson.MustParse(`"baz"`),
	},
	{
		"two names: dotted",
		fastjson.MustParse(`{"foo.bar": "baz"}`),
		[]string{"foo", "bar"},
		fastjson.MustParse(`"baz"`),
	},
	{
		"three names: 1",
		fastjson.MustParse(`{"a": {"b": {"c": "d"}}}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
	},
	{
		"three names: 2",
		fastjson.MustParse(`{"a.b": {"c": "d"}}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
	},
	{
		"three names: 3",
		fastjson.MustParse(`{"a.b.c": "d"}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
	},
	{
		"three names: 4",
		fastjson.MustParse(`{"a": {"b.c": "d"}}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
	},
	{
		"four names",
		fastjson.MustParse(`{"a.b": {"c.d": "e"}}`),
		[]string{"a", "b", "c", "d"},
		fastjson.MustParse(`"e"`),
	},
	{
		"three names: nope",
		fastjson.MustParse(`{"a": {"b": {"c": "d"}}}`),
		[]string{"a", "b", "nope"},
		nil,
	},
	{
		"val type: obj",
		fastjson.MustParse(`{"a": {"b": {"an": "obj"}}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`{"an": "obj"}`),
	},
	{
		"val type: array",
		fastjson.MustParse(`{"a": {"b": [1, 2]}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`[1, 2]`),
	},
	{
		"val type: number",
		fastjson.MustParse(`{"a": {"b": 42}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`42`),
	},
	{
		"val type: true",
		fastjson.MustParse(`{"a": {"b": true}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`true`),
	},
	{
		"val type: false",
		fastjson.MustParse(`{"a": {"b": false}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`false`),
	},
	{
		"val type: null",
		fastjson.MustParse(`{"a": {"b": null}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`null`),
	},
	{
		"non-object obj",
		fastjson.MustParse(`[1, 2, 3]`),
		[]string{"0"},
		nil,
	},
	{
		"empty string names in lookup",
		fastjson.MustParse(`{"a": {"": {"c": "d"}}}`),
		[]string{"a", "", "c"}, // a..c
		fastjson.MustParse(`"d"`),
	},
}

func equalVal(a, b *fastjson.Value) bool {
	if a == nil {
		return b == nil
	} else if b == nil {
		return false
	} else {
		return a.String() == b.String()
	}
}

func TestLookup(t *testing.T) {
	for _, tc := range lookupTestCases {
		debugf("-- lookup test case %q\n", tc.name)
		debugf("  obj: %s\n", tc.obj)
		debugf("  lookup: %s\n", tc.lookup)
		val := lookupValue(tc.obj, tc.lookup)
		debugf("  val: %s\n", val)
		if !equalVal(val, tc.val) {
			t.Errorf(
				"%s:\n"+
					"obj:\n"+
					"\t%s\n"+
					"lookup:\n"+
					"\t%+v\n"+
					"got val:\n"+
					"\t%+v\n"+
					"expected val:\n"+
					"\t%v\n",
				tc.name, tc.obj, tc.lookup, val, tc.val)
		}
	}
}

package jsonutils

import (
	"testing"

	"github.com/valyala/fastjson"
)

type lookupTestCase struct {
	name            string
	obj             *fastjson.Value
	lookup          []string
	val             *fastjson.Value
	objAfterExtract *fastjson.Value
}

var lookupTestCases = []lookupTestCase{
	{
		"empty object",
		fastjson.MustParse(`{}`),
		[]string{"foo"},
		nil,
		fastjson.MustParse(`{}`),
	},
	{
		"nil obj",
		nil,
		[]string{"foo"},
		nil,
		nil,
	},
	{
		"single name",
		fastjson.MustParse(`{"foo": "bar"}`),
		[]string{"foo"},
		fastjson.MustParse(`"bar"`),
		fastjson.MustParse(`{}`),
	},
	{
		"two names: nested",
		fastjson.MustParse(`{"foo": {"bar": "baz"}}`),
		[]string{"foo", "bar"},
		fastjson.MustParse(`"baz"`),
		fastjson.MustParse(`{}`),
	},
	{
		"two names: dotted",
		fastjson.MustParse(`{"foo.bar": "baz"}`),
		[]string{"foo", "bar"},
		fastjson.MustParse(`"baz"`),
		fastjson.MustParse(`{}`),
	},
	{
		"three names: 1",
		fastjson.MustParse(`{"a": {"b": {"c": "d"}}}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
		fastjson.MustParse(`{}`),
	},
	{
		"three names: 2",
		fastjson.MustParse(`{"a.b": {"c": "d"}}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
		fastjson.MustParse(`{}`),
	},
	{
		"three names: 3",
		fastjson.MustParse(`{"a.b.c": "d"}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
		fastjson.MustParse(`{}`),
	},
	{
		"three names: 4",
		fastjson.MustParse(`{"a": {"b.c": "d"}}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
		fastjson.MustParse(`{}`),
	},
	{
		"three names: 5",
		fastjson.MustParse(`{"a": {"b.c": "d", "e": "f"}}`),
		[]string{"a", "b", "c"},
		fastjson.MustParse(`"d"`),
		fastjson.MustParse(`{"a": {"e": "f"}}`),
	},
	{
		"four names",
		fastjson.MustParse(`{"a.b": {"c.d": "e"}}`),
		[]string{"a", "b", "c", "d"},
		fastjson.MustParse(`"e"`),
		fastjson.MustParse(`{}`),
	},
	{
		"three names: nope",
		fastjson.MustParse(`{"a": {"b": {"c": "d"}}}`),
		[]string{"a", "b", "nope"},
		nil,
		fastjson.MustParse(`{"a": {"b": {"c": "d"}}}`),
	},
	{
		"val type: obj",
		fastjson.MustParse(`{"a": {"b": {"an": "obj"}}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`{"an": "obj"}`),
		fastjson.MustParse(`{}`),
	},
	{
		"val type: array",
		fastjson.MustParse(`{"a": {"b": [1, 2]}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`[1, 2]`),
		fastjson.MustParse(`{}`),
	},
	{
		"val type: number",
		fastjson.MustParse(`{"a": {"b": 42}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`42`),
		fastjson.MustParse(`{}`),
	},
	{
		"val type: true",
		fastjson.MustParse(`{"a": {"b": true}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`true`),
		fastjson.MustParse(`{}`),
	},
	{
		"val type: false",
		fastjson.MustParse(`{"a": {"b": false}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`false`),
		fastjson.MustParse(`{}`),
	},
	{
		"val type: null",
		fastjson.MustParse(`{"a": {"b": null}}`),
		[]string{"a", "b"},
		fastjson.MustParse(`null`),
		fastjson.MustParse(`{}`),
	},
	{
		"non-object obj",
		fastjson.MustParse(`[1, 2, 3]`),
		[]string{"0"},
		nil,
		fastjson.MustParse(`[1, 2, 3]`),
	},
	{
		"empty string names in lookup",
		fastjson.MustParse(`{"a": {"": {"c": "d"}}}`),
		[]string{"a", "", "c"}, // a..c
		fastjson.MustParse(`"d"`),
		fastjson.MustParse(`{}`),
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

func TestLookupValue(t *testing.T) {
	for _, tc := range lookupTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("  obj: %s\n", tc.obj)
			t.Logf("  lookup: %s\n", tc.lookup)
			val := LookupValue(tc.obj, tc.lookup...)
			t.Logf("  val: %s\n", val)
			if !equalVal(val, tc.val) {
				t.Errorf(
					"%s:\n"+
						"obj:\n"+
						"\t%s\n"+
						"lookup:\n"+
						"\t%+v\n"+
						"got val:\n"+
						"\t%+v\n"+
						"want val:\n"+
						"\t%v\n",
					tc.name, tc.obj, tc.lookup, val, tc.val)
			}
		})
	}
}

func TestExtractValue(t *testing.T) {
	for _, tc := range lookupTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("  obj (before): %v\n", tc.obj)
			objReprBefore := "<nil>"
			if tc.obj != nil {
				objReprBefore = tc.obj.String()
			}
			t.Logf("  lookup: %s\n", tc.lookup)
			val := ExtractValue(tc.obj, tc.lookup...)
			t.Logf("  val: %v\n", val)
			t.Logf("  obj (after): %v\n", tc.obj)
			if !equalVal(val, tc.val) {
				t.Errorf(
					"%s:\n"+
						"obj:\n"+
						"\t%s\n"+
						"lookup:\n"+
						"\t%+v\n"+
						"got val:\n"+
						"\t%+v\n"+
						"want val:\n"+
						"\t%v\n",
					tc.name, objReprBefore, tc.lookup, val, tc.val)
			}
			if !equalVal(tc.obj, tc.objAfterExtract) {
				t.Errorf(
					"%s:\n"+
						"obj:\n"+
						"\t%s\n"+
						"lookup:\n"+
						"\t%+v\n"+
						"got obj after ExtractValue:\n"+
						"\t%+v\n"+
						"want obj after ExtractValue:\n"+
						"\t%v\n",
					tc.name, objReprBefore, tc.lookup, tc.obj, tc.objAfterExtract)
			}
		})
	}
}

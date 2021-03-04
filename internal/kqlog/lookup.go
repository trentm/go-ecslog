package kqlog

import (
	"strings"

	"github.com/valyala/fastjson"
)

// lookupValue looks up the JSON value identified by lookup.
//
// Assumption: There are no conflicts. E.g. we don't have:
//    obj:    {"foo.bar": 42, "foo": {"bar": 43}}
//    lookup: [foo, bar]
// In this case, the result is unspecified. *One* of the paths will win.
//
func lookupValue(obj *fastjson.Value, lookup []string) *fastjson.Value {
	if obj == nil {
		return nil
	} else if len(lookup) == 0 {
		return obj
	}

	o := obj.GetObject()
	if o == nil {
		return nil
	}

	if len(lookup) == 1 {
		return o.Get(lookup[0])
	}

	// Otherwise, we have multiple lookup names.
	//
	// E.g.: Given: lookup=["a", "b", "c"]
	// first try:   lookupValue(obj["a"], ["b", "c"])
	// then try:    lookupValue(obj["a.b"], ["c"])
	// then try:    lookupValue(obj["a.b.c"], [])
	var val *fastjson.Value
	var key string
	for i := 1; i <= len(lookup); i++ {
		key = strings.Join(lookup[:i], ".")
		val = lookupValue(o.Get(key), lookup[i:])
		// log.Printf("XXX i=%d < %d: o[%q] = %#v\n", i, len(lookup), key, val)
		if val != nil {
			return val
		}
	}

	return nil
}

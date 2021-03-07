package jsonutils

// Convenience functions for working with the fastjson API.

import (
	"strings"

	"github.com/valyala/fastjson"
)

// LookupValue looks up the JSON value identified by object property names in
// `lookup`.
//
// ECS allows a field "foo.bar" to be dotted:
//    {"foo.bar": 42}
// or undotted:
//    {foo": {"bar": 43}}
//
// Assumption: There are no conflicts. E.g. we don't have:
//    obj:    {"foo.bar": 42, "foo": {"bar": 43}}
//    lookup: [foo, bar]
// In this case, the result is unspecified. *One* of the paths will win.
//
func LookupValue(obj *fastjson.Value, lookup []string) *fastjson.Value {
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
	// first try:   LookupValue(obj["a"], ["b", "c"])
	// then try:    LookupValue(obj["a.b"], ["c"])
	// then try:    LookupValue(obj["a.b.c"], [])
	var val *fastjson.Value
	var key string
	for i := 1; i <= len(lookup); i++ {
		key = strings.Join(lookup[:i], ".")
		val = LookupValue(o.Get(key), lookup[i:])
		// log.Printf("XXX i=%d < %d: o[%q] = %#v\n", i, len(lookup), key, val)
		if val != nil {
			return val
		}
	}

	return nil
}

// ExtractValue looks up the JSON value identified by object property names in
// `lookup` (the same as `LookupValue`), and then *removes* that property from
// the object. If removing that property results in any empty object, then
// that object is removed as well -- except the top-level object is not
// changed to nil.
func ExtractValue(obj *fastjson.Value, lookup []string) *fastjson.Value {
	var val *fastjson.Value
	var key string

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
		val = o.Get(lookup[0])
		if val != nil {
			o.Del(lookup[0])
		}
		return val
	}

	// Otherwise, we have multiple lookup names.
	//
	// E.g.: Given: lookup=["a", "b", "c"]
	// first try:   ExtractValue(obj["a"], ["b", "c"])
	// then try:    ExtractValue(obj["a.b"], ["c"])
	// then try:    ExtractValue(obj["a.b.c"], [])
	for i := 1; i <= len(lookup); i++ {
		key = strings.Join(lookup[:i], ".")
		subO := o.Get(key)
		val = ExtractValue(subO, lookup[i:])
		if val != nil {
			if i == len(lookup) {
				// The value is a property of `o`, e.g. a lookup of "a.b.c"
				// in `{"a.b.c": 42}`.
				o.Del(key)
			} else if subO.GetObject().Len() == 0 {
				o.Del(key)
			}
			return val
		}
	}

	return nil
}

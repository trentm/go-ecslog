package kqlog

// This file holds the `rpnStep` interface and all the `rpn*` structs that
// implement it. These structs are the output of parsing in parse.go.
//
// A single `rpnStep` is a step in the Reverse Polish Notation (RPN) list of
// steps used to evaluate a KQL query against a given record (see
// `filter.Match()`).

import (
	"fmt"
	"strings"

	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/trentm/go-ecslog/internal/lg"
	"github.com/valyala/fastjson"
)

// rpnStep is a single step in the RPN series of steps used for filter matching.
type rpnStep interface {
	fmt.Stringer
	exec(stack *boolStack, rec *fastjson.Value)
}

type rpnExistsQuery struct {
	field string
}

func (q *rpnExistsQuery) exec(stack *boolStack, rec *fastjson.Value) {
	val := jsonutils.LookupValue(rec, strings.Split(q.field, ".")...)
	stack.Push(val != nil)
}
func (q rpnExistsQuery) String() string {
	return fmt.Sprintf(`rpnExistsQuery{%s:*}`, q.field)
}

type rpnTermsQuery struct {
	field    string
	terms    []term
	matchAll bool // Indicates all terms must match in an array field. E.g. `foo:(a and b and c)`.
}

func (q *rpnTermsQuery) exec(stack *boolStack, rec *fastjson.Value) {
	fieldVal := jsonutils.LookupValue(rec, strings.Split(q.field, ".")...)
	if fieldVal == nil {
		stack.Push(false)
		return
	}

	// TODO: wildcard handling in terms
	// TODO: wildcard handling in field!

	// Example: `foo:(bar and baz)` is meant to assert that both "bar" and
	// "baz" are present in the *array* "foo".
	if q.matchAll {
		if fieldVal.Type() != fastjson.TypeArray {
			stack.Push(false)
			return
		}

		lg.Printf("XXX rpnTermsQuery matchAll NYI\n")
		stack.Push(false)
		return
	}

	for _, t := range q.terms {
		switch fieldVal.Type() {
		case fastjson.TypeNull:
			if t.Val == "null" {
				stack.Push(true)
				return
			}
		case fastjson.TypeObject:
			// No term matches an object.
			stack.Push(false)
			return
		case fastjson.TypeArray:
			// No term matches an array.
			stack.Push(false)
			return
		case fastjson.TypeString:
			if doesTermMatchStringVal(t, fieldVal) {
				stack.Push(true)
				return
			}
		case fastjson.TypeNumber:
			numVal, ok := t.GetNumVal()
			if ok && numVal == fieldVal.GetFloat64() {
				stack.Push(true)
				return
			}
		case fastjson.TypeTrue:
			boolVal, ok := t.GetBoolVal()
			if ok && boolVal == true {
				stack.Push(true)
				return
			}
		case fastjson.TypeFalse:
			boolVal, ok := t.GetBoolVal()
			if ok && boolVal == false {
				stack.Push(true)
				return
			}
		}
	}
	stack.Push(false)
}

func (q rpnTermsQuery) String() string {
	var s string
	var termStrs []string
	for _, t := range q.terms {
		termStrs = append(termStrs, t.Val)
	}
	if q.matchAll {
		s = fmt.Sprintf(`rpnTermsQuery{%s:("%s")}`, q.field, strings.Join(termStrs, `" and "`))
	} else {
		s = fmt.Sprintf(`rpnTermsQuery{%s:"%s"}`, q.field, strings.Join(termStrs, `" "`))
	}
	return s
}

func doesTermMatchStringVal(t term, val *fastjson.Value) bool {
	// XXX: support wildcard
	return t.Val == string(val.GetStringBytes())
}

type rpnDefaultFieldsTermsQuery struct {
	terms []string
}

func (q *rpnDefaultFieldsTermsQuery) exec(stack *boolStack, rec *fastjson.Value) {
	lg.Println("XXX rpnDefaultFieldsTermsQuery NYI: what are the typical default fields for logs in Kibana?")
	stack.Push(false)
}
func (q rpnDefaultFieldsTermsQuery) String() string {
	return fmt.Sprintf(`rpnDefaultFieldsTermsQuery{"%s"}`, strings.Join(q.terms, `" "`))
}

type rpnGtRangeQuery struct {
	field        string
	term         term
	logLevelLess LogLevelLessFn
}

func (q *rpnGtRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	fieldVal := jsonutils.LookupValue(rec, strings.Split(q.field, ".")...)
	if fieldVal == nil {
		stack.Push(false)
		return
	}

	typ := fieldVal.Type()

	// Special case log.level.
	if q.logLevelLess != nil && q.field == "log.level" && typ == fastjson.TypeString {
		// `fieldVal > term` is `LogLevelLess(term, fieldVal)`.
		stack.Push(q.logLevelLess(
			q.term.Val,
			string(fieldVal.GetStringBytes()),
		))
		return
	}

	switch fieldVal.Type() {
	case fastjson.TypeString:
		stack.Push(string(fieldVal.GetStringBytes()) > q.term.Val)
	case fastjson.TypeNumber:
		numVal, ok := q.term.GetNumVal()
		if !ok {
			// For example, matching `foo > bar` ("bar" does not have a number
			// value) against record `{"foo": 42}`.
			lg.Printf("Q: How does Kibana handle KQL range query comparing string and number? `%s` -> %s > %s\n", q, fieldVal, q.term)
			stack.Push(false)
		} else {
			stack.Push(fieldVal.GetFloat64() > numVal)
		}
	case fastjson.TypeNull:
		lg.Printf("Q: How does Kibana handle KQL range query with null? `%s` -> %s > %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeObject:
		lg.Printf("Q: How does Kibana handle KQL range query with object? `%s` -> %s > %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeArray:
		lg.Printf("Q: How does Kibana handle KQL range query with array? `%s` -> %s > %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeTrue:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s > %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeFalse:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s > %s\n", q, fieldVal, q.term)
		stack.Push(false)
	}
}
func (q rpnGtRangeQuery) String() string {
	return fmt.Sprintf(`rpnGtRangeQuery{%s > %s}`, q.field, q.term)
}

type rpnGteRangeQuery struct {
	field        string
	term         term
	logLevelLess LogLevelLessFn
}

func (q *rpnGteRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	fieldVal := jsonutils.LookupValue(rec, strings.Split(q.field, ".")...)
	if fieldVal == nil {
		stack.Push(false)
		return
	}

	typ := fieldVal.Type()

	// Special case log.level.
	if q.logLevelLess != nil && q.field == "log.level" && typ == fastjson.TypeString {
		// `fieldVal >= term` is the same as `!(fieldVal < term)`, which is
		// `!LogLevelLess(fieldVal, term)`.
		stack.Push(!q.logLevelLess(
			string(fieldVal.GetStringBytes()),
			q.term.Val,
		))
		return
	}

	switch fieldVal.Type() {
	case fastjson.TypeString:
		stack.Push(string(fieldVal.GetStringBytes()) >= q.term.Val)
	case fastjson.TypeNumber:
		numVal, ok := q.term.GetNumVal()
		if !ok {
			// For example, matching `foo >= bar` ("bar" does not have a number
			// value) against record `{"foo": 42}`.
			lg.Printf("Q: How does Kibana handle KQL range query comparing string and number? `%s` -> %s >= %s\n", q, fieldVal, q.term)
			stack.Push(false)
		} else {
			stack.Push(fieldVal.GetFloat64() >= numVal)
		}
	case fastjson.TypeNull:
		lg.Printf("Q: How does Kibana handle KQL range query with null? `%s` -> %s >= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeObject:
		lg.Printf("Q: How does Kibana handle KQL range query with object? `%s` -> %s >= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeArray:
		lg.Printf("Q: How does Kibana handle KQL range query with array? `%s` -> %s >= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeTrue:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s >= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeFalse:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s >= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	}
}
func (q rpnGteRangeQuery) String() string {
	return fmt.Sprintf(`rpnGteRangeQuery{%s >= %s}`, q.field, q.term)
}

type rpnLtRangeQuery struct {
	field        string
	term         term
	logLevelLess LogLevelLessFn
}

func (q *rpnLtRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	fieldVal := jsonutils.LookupValue(rec, strings.Split(q.field, ".")...)
	if fieldVal == nil {
		stack.Push(false)
		return
	}

	typ := fieldVal.Type()

	// Special case log.level.
	if q.logLevelLess != nil && q.field == "log.level" && typ == fastjson.TypeString {
		// `fieldVal < term` is `LogLevelLess(fieldVal, term)`.
		stack.Push(q.logLevelLess(
			string(fieldVal.GetStringBytes()),
			q.term.Val,
		))
		return
	}

	switch fieldVal.Type() {
	case fastjson.TypeString:
		stack.Push(string(fieldVal.GetStringBytes()) < q.term.Val)
	case fastjson.TypeNumber:
		numVal, ok := q.term.GetNumVal()
		if !ok {
			// For example, matching `foo < bar` ("bar" does not have a number
			// value) against record `{"foo": 42}`.
			lg.Printf("Q: How does Kibana handle KQL range query comparing string and number? `%s` -> %s < %s\n", q, fieldVal, q.term)
			stack.Push(false)
		} else {
			stack.Push(fieldVal.GetFloat64() < numVal)
		}
	case fastjson.TypeNull:
		lg.Printf("Q: How does Kibana handle KQL range query with null? `%s` -> %s < %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeObject:
		lg.Printf("Q: How does Kibana handle KQL range query with object? `%s` -> %s < %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeArray:
		lg.Printf("Q: How does Kibana handle KQL range query with array? `%s` -> %s < %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeTrue:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s < %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeFalse:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s < %s\n", q, fieldVal, q.term)
		stack.Push(false)
	}
}
func (q rpnLtRangeQuery) String() string {
	return fmt.Sprintf(`rpnLtRangeQuery{%s < %s}`, q.field, q.term)
}

type rpnLteRangeQuery struct {
	field        string
	term         term
	logLevelLess LogLevelLessFn
}

func (q *rpnLteRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	fieldVal := jsonutils.LookupValue(rec, strings.Split(q.field, ".")...)
	if fieldVal == nil {
		stack.Push(false)
		return
	}

	typ := fieldVal.Type()

	// Special case log.level.
	if q.logLevelLess != nil && q.field == "log.level" && typ == fastjson.TypeString {
		// `fieldVal <= term` is the same as `!(term < fieldVal)` which is
		// `!LogLevelLess(term, fieldVal)`
		stack.Push(!q.logLevelLess(
			q.term.Val,
			string(fieldVal.GetStringBytes()),
		))
		return
	}

	switch fieldVal.Type() {
	case fastjson.TypeString:
		stack.Push(string(fieldVal.GetStringBytes()) <= q.term.Val)
	case fastjson.TypeNumber:
		numVal, ok := q.term.GetNumVal()
		if !ok {
			// For example, matching `foo <= bar` ("bar" does not have a number
			// value) against record `{"foo": 42}`.
			lg.Printf("Q: How does Kibana handle KQL range query comparing string and number? `%s` -> %s <= %s\n", q, fieldVal, q.term)
			stack.Push(false)
		} else {
			stack.Push(fieldVal.GetFloat64() <= numVal)
		}
	case fastjson.TypeNull:
		lg.Printf("Q: How does Kibana handle KQL range query with null? `%s` -> %s <= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeObject:
		lg.Printf("Q: How does Kibana handle KQL range query with object? `%s` -> %s <= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeArray:
		lg.Printf("Q: How does Kibana handle KQL range query with array? `%s` -> %s <= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeTrue:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s <= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	case fastjson.TypeFalse:
		lg.Printf("Q: How does Kibana handle KQL range query with bool? `%s` -> %s <= %s\n", q, fieldVal, q.term)
		stack.Push(false)
	}
}
func (q rpnLteRangeQuery) String() string {
	return fmt.Sprintf(`rpnLteRangeQuery{%s <= %s}`, q.field, q.term)
}

type rpnAnd struct{}

func (q *rpnAnd) exec(stack *boolStack, rec *fastjson.Value) {
	a := stack.Pop()
	b := stack.Pop()
	stack.Push(a && b)
}
func (q rpnAnd) String() string {
	return "rpnAnd{and}"
}

type rpnOr struct{}

func (q *rpnOr) exec(stack *boolStack, rec *fastjson.Value) {
	a := stack.Pop()
	b := stack.Pop()
	stack.Push(a || b)
}
func (q rpnOr) String() string {
	return "rpnOr{or}"
}

type rpnNot struct{}

func (q *rpnNot) exec(stack *boolStack, rec *fastjson.Value) {
	stack.Push(!stack.Pop())
}
func (q rpnNot) String() string {
	return "rpnNot{not}"
}

// rpnOpenParen is an rpnStep representing the start of a parenthesized group
// on the `ops` stack during parsing. It is never intended to be on the Filter
// steps to be `exec`d.
type rpnOpenParen struct{}

func (q *rpnOpenParen) exec(stack *boolStack, rec *fastjson.Value) {
	panic("exec'ing a rpnOpenParen")
}
func (q rpnOpenParen) String() string {
	return "rpnOpenParen{(}"
}

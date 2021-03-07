package kqlog

// Parse and eval a subset of KQL for use in ecslog for log record filtering.
//
// Usage:
//     filter, err := NewFilter("foo:bar and status >= 500")
//     if err != nil {
//         panic(err.Error())
//     }
//     if filter.Match(rec) {
//         // do something with rec
//     }

import (
	"log"
	"strings"

	"github.com/valyala/fastjson"
)

// LogLevelLessFn is a function type used by range queries for special case
// comparison of the "log.level" field.
type LogLevelLessFn func(level1, level2 string) bool

// Filter ... TODO:doc
type Filter struct {
	steps []rpnStep
}

// add appends a step to the filter.
// XXX s/add/addStep/
func (f *Filter) add(s rpnStep) {
	f.steps = append(f.steps, s)
}
func (f *Filter) addBoolOp(t token) {
	switch t.typ {
	case tokTypeAnd:
		f.add(&rpnAnd{})
	case tokTypeOr:
		f.add(&rpnOr{})
	case tokTypeNot:
		f.add(&rpnNot{})
	default:
		log.Panicf("token is not a bool op token: %s", t.typ)
	}
}

func (f Filter) String() string {
	var b strings.Builder
	b.WriteString("Filter{")
	for i, s := range f.steps {
		if i != 0 {
			b.WriteString(", ")
		}
		// Strip all but the "..." from "rpnTypeName{...}".
		sStr := s.String()
		idx := strings.IndexRune(sStr, '{')
		b.WriteString(sStr[idx+1 : len(sStr)-1])
	}
	b.WriteString("}")
	return b.String()
}

// Match returns true iff the given record matches the KQL filter.
func (f *Filter) Match(rec *fastjson.Value) bool {
	// log.Printf("-- Match with filter: %s", f) // XXX
	if len(f.steps) == 0 {
		return true
	}
	stack := make(boolStack, 0, len(f.steps)/2+1)
	for _, step := range f.steps {
		step.exec(&stack, rec)
		// log.Printf("  %35s -> %v\n", step, stack) // XXX
	}
	if len(stack) != 1 {
		log.Panicf("invalid KQL execution: stack length is not 1: %#v", stack)
	}
	return stack.Pop()
}

// NewFilter ... TODO:doc
func NewFilter(kql string, logLevelLess LogLevelLessFn) (*Filter, error) {
	p := newParser(kql, logLevelLess)
	f, err := p.parse()
	if err != nil {
		return nil, err
	}
	return f, nil
}

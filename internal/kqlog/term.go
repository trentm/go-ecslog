package kqlog

import "strconv"

// term represents a term in a parsed KQL filter expression.
//
// In a KQL like "username:bob anne and status_code >= 500 and audit:true",
// all of "bob", "anne", "500" and "true" are terms (used in the `rpn*` structs
// below).
//
// A KQL query is typically matched against many log records. When being
// compared against a number or a boolean value in a log record, the term
// needs to be converted to a number or boolean, if possible. Instead of
// repeatedly doing that for each log record, those attempted conversions
// are cached here.
type term struct {
	Val        string  // the raw string from the KQL
	numParsed  bool    // Has an attempt been made to parse the term as a num?
	numOk      bool    // Is the term a valid number?
	numVal     float64 // the term as a number
	boolParsed bool    // Has an attempt been made to parse the term as a bool?
	boolOk     bool    // Is the term a valid bool?
	boolVal    bool    // the term as a bool
}

func (t term) String() string {
	return t.Val
}

// GetBoolVal returns a boolean value for this term, if possible.
// If `ok` is true, then `boolVal` is the boolean value. If `ok` is false,
// then the term does not have a value boolean value.
func (t *term) GetBoolVal() (boolVal bool, ok bool) {
	if !t.boolParsed {
		switch t.Val {
		case "true":
			t.boolVal = true
			t.boolOk = true
		case "false":
			t.boolVal = false
			t.boolOk = true
		default:
			t.boolOk = false
		}
		t.boolParsed = true
	}
	return t.boolVal, t.boolOk
}

// GetNumVal returns a number value for this term, if possible.
// If `ok` is true, then `numVal` is the number value. If `ok` is false,
// then the term does not have a value number value.
func (t *term) GetNumVal() (numVal float64, ok bool) {
	if !t.numParsed {
		f, err := strconv.ParseFloat(t.Val, 64)
		if err != nil {
			t.numOk = false
		} else {
			t.numOk = true
			t.numVal = f
		}
		t.numParsed = true
	}
	return t.numVal, t.numOk
}

func newTerm(val string) term {
	return term{Val: val}
}

package kqlog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/trentm/go-ecslog/internal/lg"
)

// term represents a term in a parsed KQL filter expression.
//
// In a KQL like "username:bob anne and status_code >= 500 and audit:true",
// all of "bob", "anne", "500" and "true" are terms (used in the `rpn*` structs
// in rpn.go).
//
// A KQL query is typically matched against many log records. When being
// compared against a number or a boolean value in a log record, the term
// needs to be converted to a number or boolean, if possible. Instead of
// repeatedly doing that for each log record, those attempted conversions
// are cached here.
type term struct {
	Val        string         // the raw string from the KQL
	Wildcard   bool           // Are there one or more `*` in Val that represent wildcards?
	regexpVal  *regexp.Regexp // the compiled regex of Val, iff Wildcard=true
	numParsed  bool           // Has an attempt been made to parse the term as a num?
	numOk      bool           // Is the term a valid number?
	numVal     float64        // the term as a number
	boolParsed bool           // Has an attempt been made to parse the term as a bool?
	boolOk     bool           // Is the term a valid bool?
	boolVal    bool           // the term as a bool
}

func (t term) String() string {
	if t.Wildcard {
		return fmt.Sprintf("term{%q, Wildcard:%v}", t.Val, t.Wildcard)
	}
	return fmt.Sprintf("term{%q}", t.Val)
}

// MatchStringBytes returns true iff the term matches the given byte slice.
func (t *term) MatchStringBytes(b []byte) bool {
	if t.Wildcard {
		return t.regexpVal.Match(b)
	} else {
		return t.Val == string(b)
	}
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

// newTerm handles creating a `term` from an unquoted literal string.
//
// It handles escaping rules in quoted literals as defined by the `Literal`
// production in:
// https://github.com/elastic/kibana/blob/e2abb03ad03d27dcbe0a36964ea78a4361c038da/src/plugins/data/common/es_query/kuery/ast/kuery.peg#L212-L290
// See some test cases here:
// https://github.com/elastic/kibana/blob/e2abb03ad03d27dcbe0a36964ea78a4361c038da/src/plugins/data/common/es_query/kuery/ast/ast.test.ts#L316-L333
//
// If unquoted, an asterisk is a wildcard, and there can be backslash escaping for:
// - whitespace: `\t`, `\n`, `\r`
// - special characters: `\\`, `\(`, `\)`, `\:`, `\<`, `\>`, `\"`, `\{`, `\}`
// - keywords: `\and`, `\or`, `\not`
func newTerm(val string) term {
	whitespaceFromEscapeChar := map[byte]byte{
		'n': '\n',
		't': '\t',
		'r': '\r',
	}
	unquotedSpecialChars := map[byte]bool{
		'\\': true,
		'(':  true,
		')':  true,
		':':  true,
		'<':  true,
		'>':  true,
		'"':  true,
		'*':  true,
		'{':  true,
		'}':  true,
	}

	isWildcard := false
	var b strings.Builder

	// If the unescaped `*` wildcard char is found, then Val is the regexp
	// pattern, and Wildcard is set true.
	// TODO: I'm curious if KQL handles the case of a term with both a
	// unescaped and an escaped asterisk: `foo*bar\*`.
	var chunk strings.Builder
	var ch byte
	i := 0
	// A byte loop suffices here because all KQL metacharacters are ASCII.
	for i < len(val) {
		ch = val[i]
		if ch == '\\' {
			if i+1 >= len(val) {
				// In normal parsing, this is caught by the lexer. However,
				// guard against direct `newTerm("foo\\")` calls.
				lg.Fatalf("term ends in unescaped backslash (\\): %q", val)
			}
			// See if escaping whitespace, special char, or keyword.
			nextCh := val[i+1]
			if i+2 == len(val)-1 && val[i+1:] == "or" {
				chunk.WriteString("or")
				i += 2
			} else if i+3 == len(val)-1 && val[i+1:] == "and" {
				chunk.WriteString("and")
				i += 3
			} else if i+3 == len(val)-1 && val[i+1:] == "not" {
				// Must check for `\not` before checking for `\n`.
				chunk.WriteString("not")
				i += 3
			} else if ws, ok := whitespaceFromEscapeChar[nextCh]; ok {
				chunk.WriteByte(ws)
				i++
			} else if _, ok := unquotedSpecialChars[nextCh]; ok {
				chunk.WriteByte(nextCh)
				i++
			} else {
				chunk.WriteByte(ch)
			}
		} else if ch == '*' {
			isWildcard = true
			b.WriteString(regexp.QuoteMeta(chunk.String()))
			chunk.Reset()
			b.WriteString(".*")
		} else {
			chunk.WriteByte(ch)
		}
		i++
	}
	if isWildcard {
		b.WriteString(regexp.QuoteMeta(chunk.String()))
	} else {
		b.WriteString(chunk.String())
	}

	s := b.String()
	if isWildcard {
		s = "^" + s + "$"
		return term{
			Val:       s,
			Wildcard:  true,
			regexpVal: regexp.MustCompile(s),
		}
	}
	return term{
		Val:      s,
		Wildcard: false,
	}
}

// newQuotedTerm handles creating a `term` from a *quoted* literal string.
//
// It handles escaping rules in quoted literals as defined by the `Literal`
// production in:
// https://github.com/elastic/kibana/blob/e2abb03ad03d27dcbe0a36964ea78a4361c038da/src/plugins/data/common/es_query/kuery/ast/kuery.peg#L212-L290
// See some test cases here:
// https://github.com/elastic/kibana/blob/e2abb03ad03d27dcbe0a36964ea78a4361c038da/src/plugins/data/common/es_query/kuery/ast/ast.test.ts#L316-L333
//
// If quoted, there can be backslash escaping for:
// - whitespace: `\t`, `\n`, `\r`
// - special characters: `\\`, `\"`
func newQuotedTerm(val string) term {
	whitespaceFromEscapeChar := map[byte]byte{
		'n': '\n',
		't': '\t',
		'r': '\r',
	}
	unquotedSpecialChars := map[byte]bool{
		'\\': true,
		'"':  true,
	}

	if val[0] != '"' || val[len(val)-1] != '"' {
		// In normal parsing, the lexer guarantees this isn't the case. However,
		// guard this for direct newQuotedTerm usage.
		lg.Fatalf("quoted term does not include opening and closing double-quotes: %q", val)
	}

	var b strings.Builder
	var ch byte
	i := 1 // val includes the bounding double-quotes, skip them.
	// A byte loop suffices here because all KQL metacharacters are ASCII.
	for i < len(val)-1 {
		ch = val[i]
		if ch == '\\' {
			if i+1 >= len(val)-1 {
				// In normal parsing, this is caught by the lexer. However,
				// guard against direct `newQuotedTerm("foo\\")` calls.
				lg.Fatalf("quoted term ends in unescaped backslash (\\): %q", val)
			}
			// See if escaping whitespace or special char.
			nextCh := val[i+1]
			if ws, ok := whitespaceFromEscapeChar[nextCh]; ok {
				b.WriteByte(ws)
				i++
			} else if _, ok := unquotedSpecialChars[nextCh]; ok {
				b.WriteByte(nextCh)
				i++
			} else {
				b.WriteByte(ch)
			}
		} else {
			b.WriteByte(ch)
		}
		i++
	}

	return term{
		Val:      b.String(),
		Wildcard: false,
	}
}

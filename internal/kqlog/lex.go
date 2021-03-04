package kqlog

// Lex a KQL string into a channel of tokens.
// https://www.elastic.co/guide/en/kibana/current/kuery-query.html
//
// Dev Note:
// This lexer code structure is based on https://golang.org/src/text/template/parse/lex.go
// https://www.youtube.com/watch?v=HxaD_trXwRE is a talk introducing it.
// The interesting KQL-specific bits are (a) the token type `tokType`
// definitions and (b) the `lex*()` state functions.
//
// Usage:
//     l := lex(inputName, inputString)
//     for /* ... */ {
//         tok := l.nextToken()
//         // end when tok.typ == tokTypeEOF || tok.typ == tokTypeError
//     }
// where `inputName` is only used for debug/error output.
//

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokType int
type pos int // TODO: ditch the 'pos' type. I don't see the value.
type token struct {
	typ tokType
	pos pos // The position, in bytes, of this token in the input string.
	val string
}

const (
	tokTypeError tokType = iota
	tokTypeEOF
	tokTypeUnquotedLiteral
	// tokTypeSpecials is not an actual type, but used to assist String() impl.
	// Types of tokens with special meaning in KQL should be listed after here.
	tokTypeSpecials
	tokTypeOr
	tokTypeAnd
	tokTypeNot
	tokTypeOpenParen
	tokTypeCloseParen
	tokTypeColon
	tokTypeGt
	tokTypeGte
	tokTypeLt
	tokTypeLte
)

// Make the types prettyprint for testing/debugging.
var nameFromTokType = map[tokType]string{
	tokTypeError:           "error",
	tokTypeEOF:             "EOF",
	tokTypeUnquotedLiteral: "unquoted literal",
	tokTypeOr:              "or",
	tokTypeAnd:             "and",
	tokTypeNot:             "not",
	tokTypeOpenParen:       "(",
	tokTypeCloseParen:      ")",
	tokTypeColon:           ":",
	tokTypeGt:              ">",
	tokTypeGte:             ">=",
	tokTypeLt:              "<",
	tokTypeLte:             "<=",
}

func (tt tokType) String() string {
	name := nameFromTokType[tt]
	if name == "" {
		return fmt.Sprintf("token%d", int(tt))
	}
	return name
}

func (t token) String() string {
	switch {
	case t.typ == tokTypeEOF:
		return "EOF"
	case t.typ == tokTypeError:
		return fmt.Sprintf("<error: %s>", t.val)
	case t.typ > tokTypeSpecials:
		return t.val
	default:
		// TODO: want to differentiate unquoted and quoted literals here?
		return fmt.Sprintf("%q", t.val)
	}
}

const eof = -1

// lexerStateFn represents the state of the scanner.
type lexerStateFn func(*lexer) lexerStateFn

// lexer holds the state of the scanner.
type lexer struct {
	name       string     // the name of the input; used only for error reporting
	input      string     // the string being scanned
	start      pos        // the start position of this token
	pos        pos        // current position in the input
	width      pos        // width of the last rune read
	tokens     chan token // channel of scanned tokens
	parenDepth int        // nesting depth of ( )
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = pos(w)
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
}

// emit passes a token back to the client.
func (l *lexer) emit(t tokType) {
	l.tokens <- token{t, l.start, l.input[l.start:l.pos]}
	l.start = l.pos
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) lexerStateFn {
	l.tokens <- token{tokTypeError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// nextToken returns the next token from the input.
// Called by the parser, not in the lexing goroutine.
func (l *lexer) nextToken() token {
	return <-l.tokens
}

// drain drains the output so the lexing goroutine will exit.
// Called by the parser, not in the lexing goroutine.
func (l *lexer) drain() {
	for range l.tokens {
	}
}

// lex creates a new scanner for the input string.
func lex(name, input string) *lexer {
	l := &lexer{
		name:   name,
		input:  input,
		tokens: make(chan token),
	}
	go l.run()
	return l
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for state := lexInsideKQL; state != nil; {
		state = state(l)
	}
	close(l.tokens)
}

// state functions

func lexInsideKQL(l *lexer) lexerStateFn {
	switch r := l.next(); {
	case r == eof:
		// Correctly reached EOF.
		switch l.parenDepth {
		case 0:
			l.emit(tokTypeEOF)
			return nil
		case 1:
			return l.errorf("unclosed open parenthesis")
		default:
			return l.errorf("unclosed open parentheses (%d)", l.parenDepth)
		}
	case isSpace(r):
		l.ignore()
	case r == '(':
		l.emit(tokTypeOpenParen)
		l.parenDepth++
	case r == ')':
		l.emit(tokTypeCloseParen)
		l.parenDepth--
		if l.parenDepth < 0 {
			return l.errorf("unmatched close parenthesis")
		}
	case r == ':':
		l.emit(tokTypeColon)
	case r == '"':
		return l.errorf("do not yet support quoted literals")
	case r == '<':
		if l.next() == '=' {
			l.emit(tokTypeLte)
		} else {
			l.backup()
			l.emit(tokTypeLt)
		}
	case r == '>':
		if l.next() == '=' {
			l.emit(tokTypeGte)
		} else {
			l.backup()
			l.emit(tokTypeGt)
		}
	case r == '{' || r == '}':
		return l.errorf("do not support KQL nest field queries: %q", r)
	// JSON strings may not contain embedded null characters, not even escaped
	// ones. All other Unicode codepoints U+0001 through U+10FFFF are allowed.
	case '\u0001' <= r && r <= unicode.MaxRune:
		l.backup()
		return lexUnquotedLiteralOrBoolOp
	default:
		return l.errorf("unrecognized character: %#U", r)
	}
	return lexInsideKQL
}

// lexUnquotedLiteralOrBoolOp scans an unquoted literal or one of the boolean
// operators "not", "or", or "and".
func lexUnquotedLiteralOrBoolOp(l *lexer) lexerStateFn {
Loop:
	for {
		switch r := l.next(); {
		case r == eof:
			break Loop
		case isSpace(r):
			l.backup()
			break Loop
		case r == '\\':
			if l.next() == eof {
				return l.errorf("unterminated character escape")
			}
		case isDelimitingSpecialChar(r):
			l.backup()
			break Loop
		}
	}

	val := l.input[l.start:l.pos]
	if len(val) > 3 {
		l.emit(tokTypeUnquotedLiteral)
	} else {
		val = strings.ToLower(val)
		switch val {
		case "or":
			l.emit(tokTypeOr)
		case "and":
			l.emit(tokTypeAnd)
		case "not":
			l.emit(tokTypeNot)
		default:
			l.emit(tokTypeUnquotedLiteral)
		}
	}

	return lexInsideKQL
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

// From the KQL PEG:
//     SpecialCharacter
//       = [\\():<>"*{}]
// However, we've already handled '\\', and '*' is fine in an unquoted literal.
func isDelimitingSpecialChar(r rune) bool {
	return r == '(' ||
		r == ')' ||
		r == ':' ||
		r == '<' ||
		r == '>' ||
		r == '"' ||
		r == '{' ||
		r == '}'
}

package kqlog

import (
	"fmt"
	"log"
	"strings"

	"github.com/valyala/fastjson"
)

// Parsing of a kqlog string to a `Filter` object that can be executed on given
// log records.
//
// Usage:
//     p := newParser(kql)
//     filter, err := p.parse()
//     // use the `filter` (see type Filter in kqlog.go)
//

// rpnStep is a single step in the RPN series of steps used for filter matching.
type rpnStep interface {
	fmt.Stringer
	exec(stack *boolStack, rec *fastjson.Value)
}

type rpnExistsQuery struct {
	field string
}

func (q *rpnExistsQuery) exec(stack *boolStack, rec *fastjson.Value) {
	val := lookupValue(rec, strings.Split(q.field, "."))
	stack.Push(val != nil)
}
func (q rpnExistsQuery) String() string {
	return fmt.Sprintf(`rpnExistsQuery{%s:*}`, q.field)
}

type rpnTermsQuery struct {
	field    string
	terms    []string
	matchAll bool // Indicates all terms must match in an array field. E.g. `foo:(a and b and c)`.
}

func (q *rpnTermsQuery) exec(stack *boolStack, rec *fastjson.Value) {
	// XXX HERE
	// 		tail -1 ./demo.log | go run ./cmd/ecslog -q 'str:string'
	// - start test cases for these
	// - finish each of the types below
	// - add wildcard support
	// - move on to other types

	// XXX test cases for the core fields that have been *extracted* from rec.
	// 		 cat ./demo.log | go run ./cmd/ecslog -q 'log.level:info'
	// 		Ugh. These are on the *Renderer*. ... so we need either:
	// 		1. exec() to pass these values in, or
	// 		2. do not extract these fields from rec. at least not yet.
	// 		TODO: do NOT *extract* those fields until formatting (after kqlFiltering)
	val := lookupValue(rec, strings.Split(q.field, "."))
	if val == nil {
		stack.Push(false)
		return
	}

	// TODO: wildcard handling in terms
	// TODO: wildcard handling in field!

	// Example: `foo:(bar and baz)` is meant to assert that both "bar" and
	// "baz" are present in the *array* "foo".
	if q.matchAll {
		if val.Type() != fastjson.TypeArray {
			stack.Push(false)
			return
		}

		// XXX continue this matching
		stack.Push(false)
		return
	}

	for _, term := range q.terms {
		switch val.Type() {
		case fastjson.TypeNull:
			panic("XXX val null")
		case fastjson.TypeObject:
			panic("XXX val object")
		case fastjson.TypeArray:
			panic("XXX val array")
		case fastjson.TypeString:
			if doesTermMatchStringVal(term, val) {
				stack.Push(true)
				return
			}
		case fastjson.TypeNumber:
			// XXX HERE
			// - get term as float64
			// if doesTermMatchNumberVal(term, val) {
			// 	stack.Push(true)
			// 	return
			// }
			panic("XXX val num")
		case fastjson.TypeTrue:
			panic("XXX val true")
		case fastjson.TypeFalse:
			panic("XXX val false")
		}
	}
	stack.Push(false)
}
func (q rpnTermsQuery) String() string {
	var s string
	if q.matchAll {
		s = fmt.Sprintf(`rpnTermsQuery{%s:("%s")}`, q.field, strings.Join(q.terms, `" and "`))
	} else {
		s = fmt.Sprintf(`rpnTermsQuery{%s:"%s"}`, q.field, strings.Join(q.terms, `" "`))
	}
	return s
}

// XXX move all the rpn* types out to a separate exec.go or something
func doesTermMatchStringVal(term string, val *fastjson.Value) bool {
	// TODO: support wildcard
	return term == string(val.GetStringBytes())
}

// func doesTermMatchNumberVal(term string, val *fastjson.Value) bool {
// 	termNum := XXX
// }

type rpnDefaultFieldsTermsQuery struct {
	terms []string
}

func (q *rpnDefaultFieldsTermsQuery) exec(stack *boolStack, rec *fastjson.Value) {
	//XXX impl lookup and compare
	stack.Push(true)
}
func (q rpnDefaultFieldsTermsQuery) String() string {
	return fmt.Sprintf(`rpnDefaultFieldsTermsQuery{"%s"}`, strings.Join(q.terms, `" "`))
}

type rpnGtRangeQuery struct {
	field string
	value string
}

func (q *rpnGtRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	//XXX impl lookup and compare
	stack.Push(true)
}
func (q rpnGtRangeQuery) String() string {
	return fmt.Sprintf(`rpnGtRangeQuery{%s > %s}`, q.field, q.value)
}

type rpnGteRangeQuery struct {
	field string
	value string
}

func (q *rpnGteRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	//XXX impl lookup and compare
	stack.Push(true)
}
func (q rpnGteRangeQuery) String() string {
	return fmt.Sprintf(`rpnGteRangeQuery{%s >= %s}`, q.field, q.value)
}

type rpnLtRangeQuery struct {
	field string
	value string
}

func (q *rpnLtRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	//XXX impl lookup and compare
	stack.Push(true)
}
func (q rpnLtRangeQuery) String() string {
	return fmt.Sprintf(`rpnLtRangeQuery{%s < %s}`, q.field, q.value)
}

type rpnLteRangeQuery struct {
	field string
	value string
}

func (q *rpnLteRangeQuery) exec(stack *boolStack, rec *fastjson.Value) {
	//XXX impl lookup and compare
	stack.Push(true)
}
func (q rpnLteRangeQuery) String() string {
	return fmt.Sprintf(`rpnLteRangeQuery{%s <= %s}`, q.field, q.value)
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

type parser struct {
	kql              string // the KQL text being parsed
	lex              *lexer
	lookAheadTok     *token     // a lookahead token, if peek() or backup() was called
	stagedOps        tokenStack // a stack of staged bool ops in increasing order of precedence (and open parens)
	field            *token     // the field name token during the parse of a query
	incompleteBoolOp bool       // true if a boolean operator has been parsed, but the following query has not yet been parsed
	filter           *Filter    // the Filter to be returned
	err              error      // ... or an error to return instead.
}

// stageBoolOp handles a parsed boolean operator (and, or, not).
//
// tl;dr: Pop operators already staged, stopping at an operator of lower
// precedence, then stage this op on the stack.
//
// Because we are building steps in RPN, the boolean operator (e.g. `or` in
// `foo or bar`) cannot be added to the steps until *after* its operand(s).
// Hence staging this op.
//
// In addition, operators already on the stack should possibly be added to the
// steps when this boolean operator is parsed. E.g., `foo and bar or baz` should
// result in the RPN steps `foo, bar, and, baz, or`. When `or` is parsed, `and`
// is already staged:
//     foo and bar or baz                    steps: foo, bar
//                 ^                     stagedOps: and
// `and` is higher precedence, so it is popped:
//     foo and bar or baz                    steps: foo, bar, and
//                    ^                  stagedOps: or
func (p *parser) stageBoolOp(opTok token) {
	precedence := opPrecedenceFromTokType[opTok.typ]
	for p.stagedOps.Len() > 0 {
		top := p.stagedOps.Peek()
		if top.typ == tokTypeOpenParen {
			// Stop at an open-paren: only a close-paren pops an open-paren.
			break
		} else if opPrecedenceFromTokType[top.typ] >= precedence {
			p.filter.addBoolOp(p.stagedOps.Pop())
		} else {
			break
		}
	}
	p.stagedOps.Push(opTok)
}

// opPrecedenceFromTokType defines boolean operator precendence. Used by
// `stageBoolOp`.
var opPrecedenceFromTokType = map[tokType]int{
	tokTypeOr:  1, // lowest
	tokTypeAnd: 2,
	tokTypeNot: 3, // highest
}

// next returns the next lexer token.
func (p *parser) next() token {
	var tok token
	if p.lookAheadTok != nil {
		tok = *p.lookAheadTok
		p.lookAheadTok = nil
	} else {
		tok = p.lex.nextToken()
	}
	return tok
}

// peek returns the next lexer token, but does not consume it.
// Can only peek ahead one token.
func (p *parser) peek() token {
	if p.lookAheadTok != nil {
		panic("cannot parser.peek(), lookAheadTok is in used")
	}
	tok := p.lex.nextToken()
	p.lookAheadTok = &tok
	return tok
}

// backup backs up one token.
func (p *parser) backup(tok token) {
	if p.lookAheadTok != nil {
		panic("cannot parser.backup(), lookAheadTok is in used")
	}
	p.lookAheadTok = &tok
}

type parserStateFn func(*parser) parserStateFn

func (p *parser) errorfAt(pos pos, format string, args ...interface{}) parserStateFn {
	ctx := fmt.Sprintf("\n    %s\n    %s^", p.kql, strings.Repeat(".", int(pos)))
	p.err = fmt.Errorf(format+ctx, args...)
	return nil
}

func parseErrorTok(p *parser) parserStateFn {
	tok := p.next()
	return p.errorfAt(tok.pos, "%s", tok.val)
}

// parseRangeQuery parses one of the range queries. `p.field` holds the field
// name token, and the next token is already checked to be one of the range
// operator token types (e.g. tokTypeGt).
func parseRangeQuery(p *parser) parserStateFn {
	opTok := p.next() // Already checked to be the range operator token.
	valTok := p.next()
	switch valTok.typ {
	case tokTypeError:
		p.backup(valTok)
		return parseErrorTok
	case tokTypeUnquotedLiteral:
		var q rpnStep
		switch opTok.typ {
		case tokTypeGt:
			q = &rpnGtRangeQuery{field: p.field.val, value: valTok.val}
		case tokTypeGte:
			q = &rpnGteRangeQuery{field: p.field.val, value: valTok.val}
		case tokTypeLt:
			q = &rpnLtRangeQuery{field: p.field.val, value: valTok.val}
		case tokTypeLte:
			q = &rpnLteRangeQuery{field: p.field.val, value: valTok.val}
		default:
			log.Panicf("invalid opTok.typ=%v while parsing range query", opTok.typ)
		}
		p.filter.add(q)
		p.field = nil
		return parseAfterQuery
	default:
		return p.errorfAt(valTok.pos, "expected a literal after '%s'; got %s",
			opTok, valTok.typ)
	}
}

// parseTermsQuery parses one of the types of "terms queries". The field token
// has been parsed to `p.field` and the next token is the colon.
//
// E.g.: `foo:value1 value2`, `foo:(a or b)`, `foo:(a and b and c)`, `foo:*`
func parseTermsQuery(p *parser) parserStateFn {
	p.next() // Consume the ':' token.
	var terms []string
	tok := p.next()
	switch tok.typ {
	case tokTypeUnquotedLiteral:
		// E.g. `foo:val1 val2` or `foo:*`. If at least on of the terms is `*`,
		// then this is an "exists query".
		terms = append(terms, tok.val)
		haveExistsTerm := tok.val == "*"
		for {
			tok := p.peek()
			if tok.typ == tokTypeUnquotedLiteral {
				if tok.val == "*" {
					haveExistsTerm = true
				}
				terms = append(terms, tok.val)
				p.next() // Consume the token.
			} else {
				break
			}
		}
		if haveExistsTerm {
			p.filter.add(&rpnExistsQuery{field: p.field.val})
		} else {
			p.filter.add(&rpnTermsQuery{field: p.field.val, terms: terms})
		}
		p.field = nil
		return parseAfterQuery
	case tokTypeOpenParen:
		// E.g. `foo:(a or b ...)` or `foo:(a and b and c)`.
		//
		// TODO: Edge cases like no terms `foo:()`, a single term `foo:(a)`,
		// superfluous parentheses `foo:((a and (b)))`, wildcard in second
		// form `foo:(a and *)`.
		matchAll := false // True if the second form with `and`: `foo:(a and b ...)`.
		haveExistsTerm := false
		for i := 0; true; i++ {
			// Expect literal ...
			termTok := p.next()
			if termTok.typ != tokTypeUnquotedLiteral {
				return p.errorfAt(termTok.pos, "expected literal, got %s", termTok.typ)
			}
			terms = append(terms, termTok.val)
			if termTok.val == "*" {
				haveExistsTerm = true
			}
			// ... then ')' to complete the query, or 'and' or 'or' to repeat.
			opTok := p.next()
			switch opTok.typ {
			case tokTypeCloseParen:
				if haveExistsTerm {
					// For now, treating `*` literal in these forms as an
					// exists query. TODO: verify against kuery.peg.
					p.filter.add(&rpnExistsQuery{field: p.field.val})
				} else {
					p.filter.add(&rpnTermsQuery{field: p.field.val, terms: terms, matchAll: matchAll})
				}
				p.field = nil
				return parseAfterQuery
			case tokTypeOr:
				if i == 0 {
					matchAll = false
				} else if matchAll {
					return p.errorfAt(opTok.pos,
						"cannot mix 'and' and 'or' in parenthesized value group")
				}
			case tokTypeAnd:
				if i == 0 {
					matchAll = true
				} else if !matchAll {
					return p.errorfAt(opTok.pos,
						"cannot mix 'and' and 'or' in parenthesized value group")
				}
			default:
				return p.errorfAt(opTok.pos, "expected ')', 'or', or 'and'; got %s",
					opTok.typ)
			}
		}
		panic(fmt.Sprintf("unreachable code hit with KQL %q", p.kql))
	default:
		return p.errorfAt(tok.pos, "expected a literal or '('; got %s", tok.typ)
	}
}

// parseAfterQuery handles parsing of tokens after a query has been parsed.
// See `parseBeforeQuery` for what is meant as a "query" here.
func parseAfterQuery(p *parser) parserStateFn {
	tok := p.next()
	switch tok.typ {
	case tokTypeError:
		p.backup(tok)
		return parseErrorTok
	case tokTypeEOF:
		p.backup(tok)
		return parseEOFTok
	case tokTypeCloseParen:
		if p.incompleteBoolOp {
			// E.g.: "(foo and)"
			return p.errorfAt(tok.pos, "incomplete boolean operator")
		}
		// Pop ops up to, and including, the matching rpnOpenParen.
		for {
			if p.stagedOps.Len() == 0 {
				return p.errorfAt(tok.pos, "unmatched close parenthesis")
			}
			opTok := p.stagedOps.Pop()
			if opTok.typ == tokTypeOpenParen {
				break
			} else {
				p.filter.addBoolOp(opTok)
			}
		}
		return parseAfterQuery
	case tokTypeAnd:
		p.stageBoolOp(tok)
		p.incompleteBoolOp = true
		return parseBeforeQuery
	case tokTypeOr:
		p.stageBoolOp(tok)
		p.incompleteBoolOp = true
		return parseBeforeQuery
	default:
		return p.errorfAt(tok.pos, "expect 'and', 'or', or ')'; got %s",
			tok.typ)
	}
}

// parseEOFTok handles completing parsing on the EOF token.
func parseEOFTok(p *parser) parserStateFn {
	tok := p.next()
	if tok.typ != tokTypeEOF {
		log.Panicf("parseEOFTok called with token other than EOF: '%s'", tok.typ)
	}
	if p.incompleteBoolOp {
		// E.g.: "foo and"
		return p.errorfAt(tok.pos, "incomplete boolean operator")
	}
	// Append all remaining staged ops.
	// Note: Lexing already handles unclosed open parens, so we need not check
	// that here.
	for p.stagedOps.Len() > 0 {
		p.filter.addBoolOp(p.stagedOps.Pop())
	}
	return nil
}

// parseBeforeQuery handles parsing of tokens at the start of a query, by
// which we mean any of the single query set of tokens except the boolean
// queries.
//
// For example, in the following the underlined are the "queries" we mean:
//     a.field:value and (not another.field > 42 or yet.another.field:"blarg")
//     -------------          ------------------    -------------------------
func parseBeforeQuery(p *parser) parserStateFn {
	tok := p.next()
	switch tok.typ {
	case tokTypeError:
		p.backup(tok)
		return parseErrorTok
	case tokTypeEOF:
		p.backup(tok)
		return parseEOFTok
	case tokTypeOpenParen:
		// Push the '(' onto the ops stack. It will be the marker at which to
		// stop when the ')' token is parsed.
		p.stagedOps.Push(tok)
		return parseBeforeQuery
	case tokTypeNot:
		p.stageBoolOp(tok)
		p.incompleteBoolOp = true
		return parseBeforeQuery
	case tokTypeUnquotedLiteral:
		p.incompleteBoolOp = false
		switch tok2 := p.peek(); tok2.typ {
		case tokTypeError:
			return parseErrorTok
		case tokTypeGt, tokTypeGte, tokTypeLt, tokTypeLte:
			// E.g.: `a.field >= 100`, `some.date.field < "2021-02"`
			p.field = &tok
			return parseRangeQuery
		case tokTypeColon:
			// E.g.: `foo:value1 value2`, `foo:(a or b)`, `foo:(a and b and c)`,
			// `foo:*`
			p.field = &tok
			return parseTermsQuery
		default:
			// E.g.: `foo bar baz`
			// No range operator and no colon means this is a query without
			// a field name. In Kibana, this matches against "default fields".
			var termTok token
			terms := []string{tok.val}
			for {
				termTok = p.next()
				if termTok.typ == tokTypeUnquotedLiteral {
					terms = append(terms, termTok.val)
				} else {
					break
				}
			}
			p.backup(termTok)
			p.filter.add(&rpnDefaultFieldsTermsQuery{terms: terms})
			return parseAfterQuery
		}
	default:
		return p.errorfAt(tok.pos,
			"expecting a literal, 'not', or '('; got %s", tok.typ)
	}
}

func (p *parser) parse() (*Filter, error) {
	for state := parseBeforeQuery; state != nil; {
		state = state(p)
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.filter, nil
}

func newParser(kql string) *parser {
	return &parser{
		kql:       kql,
		lex:       lex("NewFilter", kql),
		stagedOps: make(tokenStack, 0),
		filter:    &Filter{},
	}
}

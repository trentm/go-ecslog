package kqlog

// Parsing of a kqlog string to a `Filter` object that can be executed on given
// log records.
//
// Usage:
//     kql := "... some KQL string ..."
//     func logLevelLess(level1, level2 string) bool {
//         // ...
//     }
//
//     p := newParser(kql, logLevelLess)
//     filter, err := p.parse()
//     // use the `filter` (see type Filter in kqlog.go)

import (
	"fmt"
	"strings"

	"github.com/trentm/go-ecslog/internal/lg"
)

type parser struct {
	kql              string         // the KQL text being parsed
	logLevelLess     LogLevelLessFn // an optional fn to special case "log.level" range queries
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
			q = &rpnGtRangeQuery{
				field:        p.field.val,
				term:         newTerm(valTok.val),
				logLevelLess: p.logLevelLess,
			}
		case tokTypeGte:
			q = &rpnGteRangeQuery{
				field:        p.field.val,
				term:         newTerm(valTok.val),
				logLevelLess: p.logLevelLess,
			}
		case tokTypeLt:
			q = &rpnLtRangeQuery{
				field:        p.field.val,
				term:         newTerm(valTok.val),
				logLevelLess: p.logLevelLess,
			}
		case tokTypeLte:
			q = &rpnLteRangeQuery{
				field:        p.field.val,
				term:         newTerm(valTok.val),
				logLevelLess: p.logLevelLess,
			}
		default:
			lg.Fatalf("invalid opTok.typ=%v while parsing range query", opTok.typ)
		}
		p.filter.addStep(q)
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
	var terms []term
	tok := p.next()
	switch tok.typ {
	case tokTypeUnquotedLiteral:
		// E.g. `foo:val1 val2` or `foo:*`. If at least on of the terms is `*`,
		// then this is an "exists query".
		terms = append(terms, newTerm(tok.val))
		haveExistsTerm := tok.val == "*"
		for {
			tok := p.peek()
			if tok.typ == tokTypeUnquotedLiteral {
				if tok.val == "*" {
					haveExistsTerm = true
				}
				terms = append(terms, newTerm(tok.val))
				p.next() // Consume the token.
			} else {
				break
			}
		}
		if haveExistsTerm {
			p.filter.addStep(&rpnExistsQuery{field: p.field.val})
		} else {
			p.filter.addStep(&rpnTermsQuery{field: p.field.val, terms: terms})
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
			terms = append(terms, newTerm(termTok.val))
			if termTok.val == "*" {
				haveExistsTerm = true
			}
			// ... then ')' to complete the query, or 'and' or 'or' to repeat.
			opTok := p.next()
			switch opTok.typ {
			case tokTypeCloseParen:
				if haveExistsTerm {
					// For now, treating `*` literal in these forms as an
					// exists query.
					// TODO: verify against kuery.peg, I'm not so sure about this
					p.filter.addStep(&rpnExistsQuery{field: p.field.val})
				} else if matchAll {
					p.filter.addStep(&rpnMatchAllTermsQuery{field: p.field.val, terms: terms})
				} else {
					p.filter.addStep(&rpnTermsQuery{field: p.field.val, terms: terms})
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
		lg.Fatalf("parseEOFTok called with token other than EOF: '%s'", tok.typ)
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
			p.filter.addStep(&rpnDefaultFieldsTermsQuery{terms: terms})
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

func newParser(kql string, loglevelLess LogLevelLessFn) *parser {
	return &parser{
		kql:          kql,
		lex:          lex("NewFilter", kql),
		stagedOps:    make(tokenStack, 0),
		filter:       &Filter{},
		logLevelLess: loglevelLess,
	}
}

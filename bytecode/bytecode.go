package graphql

import (
	"errors"
)

type parserCtx struct {
	res    []byte
	query  []byte
	charNr int
	errors []error
}

func (ctx *parserCtx) parseQueryToBytecode() {
	*ctx = parserCtx{
		res:    ctx.res[:0],
		query:  ctx.query,
		errors: ctx.errors[:0],
	}

	for {
		if ctx.parseOperatorOrFragment() {
			return
		}
	}
}

// - http://spec.graphql.org/June2018/#sec-Language.Operations
// - http://spec.graphql.org/June2018/#FragmentDefinition
func (ctx *parserCtx) parseOperatorOrFragment() (stop bool) {
	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return true
	}

	if c == '{' {
		ctx.instructionNewOperation(operatorQuery)
	} else if matches := ctx.matches("query", "mutation", "subscription"); matches != -1 {
		// Set the operation kind
		if matches == 0 {
			ctx.instructionNewOperation(operatorQuery)
		} else if matches == 1 {
			ctx.instructionNewOperation(operatorMutation)
		} else {
			ctx.instructionNewOperation(operatorSubscription)
		}

		// Parse operation name
		_, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		_, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}

		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c != '{' {
			return ctx.err(`expected selection set opener ("{") but got "` + string(c) + `"`)
		}
	} else if matches := ctx.matches("fragment"); matches != -1 {
		ctx.instructionNewFragment()

		// Parse fragment name
		_, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		empty, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if empty {
			return ctx.err(`expected fragment name but got "` + string(ctx.currentC()) + `"`)
		}

		// Parse "on"
		c, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c != 'o' {
			return ctx.err(`expected "on" keyword but got "` + string(c) + `"`)
		}
		ctx.charNr++
		c, eof = ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}
		if c != 'n' {
			return ctx.err(`expected "on" keyword but got "` + string(c) + `"`)
		}
		ctx.charNr++

		// Parse fragment target type name
		_, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		ctx.res = append(ctx.res, 0)
		empty, criticalErr = ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if empty {
			return ctx.err(`expected fragment type target but got "` + string(ctx.currentC()) + `"`)
		}

		// Parse fragment body
		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c != '{' {
			return ctx.err(`expected selection set opener ("{") but got "` + string(c) + `"`)
		}
	} else {
		return ctx.err(`expected query, mutation, subscription or a simple query ("{...}") but got "` + string(c) + `"`)
	}

	ctx.charNr++
	criticalErr := ctx.parseSelectionSet()
	if criticalErr {
		return criticalErr
	}
	ctx.instructionEnd()

	return false
}

func (ctx *parserCtx) parseSelectionSet() bool {
	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}

	if c == '}' {
		ctx.charNr++
		return false
	}

	for {
		ctx.instructionNewField()

		empty, criticalError := ctx.parseAndWriteName()
		if criticalError {
			return criticalError
		}
		if empty {
			// Revert changes from ctx.instructionNewField()
			ctx.res = ctx.res[:len(ctx.res)-2]

			if ctx.matches("...") == 0 {
				// Is pointer to fragment or inline fragment
				_, eof := ctx.mightIgnoreNextTokens()
				if eof {
					return ctx.unexpectedEOF()
				}

				isInline := ctx.matches("on") == 0
				if isInline {
					c, eof := ctx.checkC(ctx.charNr)
					if !eof && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
						// This is not an inline fragment, there are name chars behind the "on" for example: "online" (starts with on)
						// Revert the changes made by the match
						ctx.charNr -= 2
						isInline = false
					} else {
						_, eof := ctx.mightIgnoreNextTokens()
						if eof {
							return ctx.unexpectedEOF()
						}
					}
				}

				ctx.instructionNewFragmentSpread(isInline)
				empyt, criticalErr := ctx.parseAndWriteName()
				if criticalErr {
					return criticalErr
				}
				c, eof = ctx.mightIgnoreNextTokens()
				if eof {
					return ctx.unexpectedEOF()
				}

				if empyt {
					if isInline {
						return ctx.err(`expected fragment type name but got char: "` + string(c) + `"`)
					} else {
						return ctx.err(`expected fragment name but got char: "` + string(c) + `"`)
					}
				}

				if isInline {
					// parse inline fragment selection set
					if c != '{' {
						return ctx.err(`expected selection set open ("{") on inline fragment but got "` + string(c) + `"`)
					}
					ctx.charNr++
					ctx.parseSelectionSet()
					ctx.instructionEnd()
					c, eof = ctx.mightIgnoreNextTokens()
					if eof {
						return ctx.unexpectedEOF()
					}
				}

				if c == '}' {
					ctx.charNr++
					return false
				}

				continue
			}

			return ctx.err(`unexpected character, expected valid name or selection closure but got: "` + string(ctx.currentC()) + `"`)
		}

		c, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}

		if c == '{' {
			ctx.charNr++

			criticalErr := ctx.parseSelectionSet()
			if criticalErr {
				return criticalErr
			}

			ctx.charNr++

			c, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}
		}

		ctx.instructionEnd()

		if c == '}' {
			ctx.charNr++
			return false
		}
	}
}

//
// ITERATOR HELPERS
//

func (ctx *parserCtx) checkC(nr int) (res byte, end bool) {
	if ctx.eof(nr) {
		return 0, true
	}
	return ctx.c(nr), false
}

func (ctx *parserCtx) c(nr int) byte {
	return ctx.query[nr]
}

func (ctx *parserCtx) eof(nr int) bool {
	return nr >= len(ctx.query)
}

func (ctx *parserCtx) currentC() byte {
	return ctx.c(ctx.charNr)
}

func (ctx *parserCtx) mightIgnoreNextTokens() (nextC byte, eof bool) {
	for {
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			return 0, true
		}

		isIgnoredChar := ctx.isIgnoredToken(c)
		if !isIgnoredChar {
			return c, false
		}

		ctx.charNr++
	}
}

// https://spec.graphql.org/June2018/#sec-Source-Text.Ignored-Tokens
func (ctx *parserCtx) isIgnoredToken(c byte) bool {
	// TODO this doesn't support unicode bomb
	return c == ' ' || c == '\t' || ctx.isLineTerminator() || ctx.isComment(true) || c == 0
}

// https://spec.graphql.org/June2018/#LineTerminator
func (ctx *parserCtx) isLineTerminator() bool {
	c := ctx.currentC()
	if c == '\n' {
		return true
	}
	if c == '\r' {
		next, _ := ctx.checkC(ctx.charNr + 1)
		if next == '\n' {
			ctx.charNr++
		}
		return true
	}
	return false
}

// https://spec.graphql.org/June2018/#Comment
func (ctx *parserCtx) isComment(parseComment bool) bool {
	if ctx.currentC() == '#' {
		if parseComment {
			ctx.parseComment()
		}
		return true
	}
	return false
}

func (ctx *parserCtx) parseComment() {
	for {
		if ctx.eof(ctx.charNr) {
			return
		}
		if ctx.isLineTerminator() {
			return
		}
		ctx.charNr++
	}
}

func (ctx *parserCtx) matches(oneOf ...string) int {
	startIdx := ctx.charNr

	if len(oneOf) == 1 {
		for {
			c, eof := ctx.checkC(ctx.charNr)
			if eof {
				ctx.charNr = startIdx
				return -1
			}
			offset := ctx.charNr - startIdx

			keyLen := len(oneOf[0])
			if oneOf[0][offset] != c {
				return -1
			} else if keyLen == offset+1 {
				ctx.charNr++
				return 0
			}

			ctx.charNr++
		}
	}

	lastChecked := ""

	for {
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			ctx.charNr = startIdx
			return -1
		}
		offset := ctx.charNr - startIdx

		for idx, key := range oneOf {
			keyLen := len(key)
			if offset < keyLen {
				if key[offset] != c {
					// Nullify value so we won't check it again
					oneOf[idx] = ""
				} else if keyLen == offset+1 {
					ctx.charNr++
					return idx
				} else {
					lastChecked = key
				}
			}
		}

		if lastChecked == "" {
			ctx.charNr = startIdx
			return -1
		}

		ctx.charNr++
	}
}

type ErrorWLocation struct {
	err    error
	line   uint
	column uint
}

func (e ErrorWLocation) Error() string {
	return e.err.Error()
}

func (ctx *parserCtx) err(err string) bool {
	line := uint(1)
	column := uint(0)
	for idx, char := range ctx.query {
		if idx == ctx.charNr {
			break
		}

		switch char {
		case '\n':
			if column == 0 && idx > 0 && ctx.query[idx-1] == '\r' {
				// don't count \r\n as 2 lines
				continue
			}
			line++
			column = 0
		case '\r':
			line++
			column = 0
		default:
			column++
		}
	}

	ctx.errors = append(ctx.errors, ErrorWLocation{
		errors.New(err),
		line,
		uint(column),
	})
	return true
}

func (ctx *parserCtx) unexpectedEOF() bool {
	return ctx.err("unexpected EOF")
}

func (ctx *parserCtx) parseAndWriteName() (empty bool, criticalError bool) {
	c, eof := ctx.checkC(ctx.charNr)
	if eof {
		return true, ctx.unexpectedEOF()
	}

	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
		ctx.res = append(ctx.res, c)
		ctx.charNr++
	} else {
		return true, false
	}

	for {
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			return false, ctx.unexpectedEOF()
		}

		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (empty && c >= '0' && c <= '9') {
			ctx.res = append(ctx.res, c)
			ctx.charNr++
			continue
		}

		return false, false
	}
}

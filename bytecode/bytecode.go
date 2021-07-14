package graphql

import (
	"encoding/hex"
	"errors"
	"unicode/utf16"
	"unicode/utf8"
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
		hasArgsFlagLocation := len(ctx.res) - 2
		directivesCountLocation := len(ctx.res) - 1

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
		if c == '(' {
			ctx.res[hasArgsFlagLocation] = 't'
			ctx.charNr++
			criticalErr := ctx.parseOperatorArguments()
			if criticalErr {
				return criticalErr
			}
			// No need re-set c here as that will be dune by parseDirectives
		}

		amount, criticalErr := ctx.parseDirectives()
		ctx.res[directivesCountLocation] = amount
		if criticalErr {
			return criticalErr
		}
		c = ctx.currentC()

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

func (ctx *parserCtx) parseOperatorArguments() bool {
	ctx.instructionNewOperationArgs()

	for {
		c, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}

		if c == ')' {
			ctx.charNr++
			ctx.instructionEnd()
			return false
		}

		// Parse `some_name` of `query a(some_var: String = "a") {`
		ctx.instructionNewOperationArg()
		empty, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if empty {
			return ctx.err(`expected argument name but got "` + string(ctx.currentC()) + `"`)
		}

		// Parse `:` of `query a(some_var: String = "a") {`
		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c != ':' {
			return ctx.err(`expected ":" name but got "` + string(ctx.currentC()) + `"`)
		}
		ctx.charNr++

		// Parse `String` of `query a(some_var: String = "a") {`
		ctx.res = append(ctx.res, 0)
		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		criticalErr = ctx.parseGraphqlTypeName(c)
		if criticalErr {
			return criticalErr
		}
		ctx.res = append(ctx.res, 0)

		// Parse `=` of query `a(some_var: String = "a") {`
		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c == ')' {
			ctx.res = append(ctx.res, 'f')
			ctx.charNr++
			ctx.instructionEnd()
			return false
		}
		if c != '=' {
			ctx.res = append(ctx.res, 'f')
			continue
		}
		ctx.res = append(ctx.res, 't')
		ctx.charNr++

		// Parse `"a"` of `query a(some_var: String = "a") {`
		_, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		criticalErr = ctx.parseInputValue()
		if criticalErr {
			return criticalErr
		}
	}
}

func (ctx *parserCtx) parseDirectives() (directivesAmount uint8, criticalErr bool) {
	for {
		c, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return directivesAmount, ctx.unexpectedEOF()
		}
		if c != '@' {
			return directivesAmount, false
		}

		if directivesAmount == 255 {
			return directivesAmount, ctx.err(`cannot have more than 255 directives`)
		}

		directivesAmount++
		ctx.charNr++
		ctx.instructionNewDirective()
		hasArgsFlag := len(ctx.res) - 1
		empty, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return directivesAmount, criticalErr
		}
		if empty {
			return directivesAmount, ctx.err(`expected directive name but got char "` + string(ctx.currentC()) + `"`)
		}

		// parse arguments
		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return directivesAmount, ctx.unexpectedEOF()
		}
		if c != '(' {
			continue
		}
		ctx.res[hasArgsFlag] = 't'
		ctx.charNr++
		criticalErr = ctx.parseAssignmentSet(')')
		if criticalErr {
			return directivesAmount, criticalErr
		}
	}
}

func (ctx *parserCtx) parseGraphqlTypeName(c byte) bool {
	var eof bool
	operationLocation := len(ctx.res)

	if c == '[' {
		ctx.res = append(ctx.res, 'l')
		ctx.charNr++

		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}

		criticalErr := ctx.parseGraphqlTypeName(c)
		if criticalErr {
			return criticalErr
		}

		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c != ']' {
			return ctx.err(`expected list closure ("]") but got "` + string(c) + `"`)
		}
		ctx.charNr++
		c, eof = ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}

		if c == '!' {
			ctx.res[operationLocation] = 'L'
			ctx.charNr++
		}

		return false
	}

	ctx.res = append(ctx.res, 'n')
	empty, criticalErr := ctx.parseAndWriteName()
	if criticalErr {
		return criticalErr
	}
	if empty {
		return ctx.err(`invalid typename char "` + string(ctx.currentC()) + `"`)
	}

	c, eof = ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}
	if c == '!' {
		ctx.res[operationLocation] = 'N'
		ctx.charNr++
	}

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
		directivesCountLocation := len(ctx.res) - 1

		empty, criticalError := ctx.parseAndWriteName()
		if criticalError {
			return criticalError
		}

		if empty {
			// Revert changes from ctx.instructionNewField()
			ctx.res = ctx.res[:len(ctx.res)-3]

			if ctx.matches("...") == 0 {
				// Is pointer to fragment or inline fragment
				_, eof := ctx.mightIgnoreNextTokens()
				if eof {
					return ctx.unexpectedEOF()
				}

				isInline := ctx.matchesWord("on") == 0
				if isInline {
					_, eof := ctx.mightIgnoreNextTokens()
					if eof {
						return ctx.unexpectedEOF()
					}
				}

				ctx.instructionNewFragmentSpread(isInline)
				directivesCountLocation := len(ctx.res) - 1

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

				if c == '@' {
					amount, criticalErr := ctx.parseDirectives()
					ctx.res[directivesCountLocation] = amount
					if criticalErr {
						return criticalErr
					}
					c = ctx.currentC()
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

				if c == ',' {
					ctx.charNr++
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

		ctx.res = append(ctx.res, 0)

		if c == ':' {
			ctx.charNr++
			_, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}

			empty, criticalErr := ctx.parseAndWriteName()
			if criticalErr {
				return criticalErr
			}
			if empty {
				return ctx.err(`unexpected character, expected nvalid name char but got "` + string(c) + `"`)
			}

			c, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}
		}

		if c == '@' {
			amount, criticalErr := ctx.parseDirectives()
			ctx.res[directivesCountLocation] = amount
			if criticalErr {
				return criticalErr
			}
			c = ctx.currentC()
		}

		if c == '(' {
			ctx.charNr++
			_, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}

			criticalErr := ctx.parseAssignmentSet(')')
			if criticalErr {
				return criticalErr
			}

			c, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}
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

		if c == ',' {
			ctx.charNr++
			c, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}
		}

		if c == '}' {
			ctx.charNr++
			return false
		}
	}
}

// http://spec.graphql.org/June2018/#sec-Language.Arguments
// http://spec.graphql.org/June2018/#ObjectValue
func (ctx *parserCtx) parseAssignmentSet(closure byte) bool {
	ctx.instructionNewValueObject()

	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}
	if c == closure {
		ctx.instructionEnd()
		ctx.charNr++
		return false
	}

	for {
		ctx.instructionStartNewValueObjectField()

		empty, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if empty {
			return ctx.err(`expected name character but got: "` + string(ctx.currentC()) + `"`)
		}

		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c != ':' {
			return ctx.err(`expected ":" but got "` + string(c) + `"`)
		}
		ctx.charNr++

		criticalErr = ctx.parseInputValue()
		if criticalErr {
			return criticalErr
		}

		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c == ',' {
			ctx.charNr++
			c, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}
		}
		if c == closure {
			ctx.instructionEnd()
			ctx.charNr++
			return false
		}
	}
}

func (ctx *parserCtx) parseInputValue() bool {
	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}

	if c == '$' {
		ctx.charNr++
		ctx.instructionNewValueVariable()
		empty, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if empty {
			return ctx.err(`variable input should have a name, got character: "` + string(ctx.currentC()) + `"`)
		}
		return false
	}

	if c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9') {
		return ctx.parseNumberInputValue()
	}

	if c == '"' {
		return ctx.parseStringInputValue()
	}

	if c == '[' {
		ctx.charNr++
		ctx.instructionNewValueList()

		c, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}

		if c == ']' {
			ctx.charNr++
			ctx.instructionEnd()
			return false
		}

		for {
			criticalErr := ctx.parseInputValue()
			if criticalErr {
				return criticalErr
			}

			c, eof := ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}

			if c == ',' {
				ctx.charNr++
				c, eof = ctx.mightIgnoreNextTokens()
				if eof {
					return ctx.unexpectedEOF()
				}
			}

			if c == ']' {
				ctx.charNr++
				ctx.instructionEnd()
				return false
			}
		}
	}

	if c == '{' {
		ctx.charNr++
		return ctx.parseAssignmentSet('}')
	}

	if c == 't' || c == 'f' {
		if matches := ctx.matchesWord("false", "true"); matches != -1 {
			ctx.instructionNewValueBoolean(matches == 1)
			return false
		}
	} else if c == 'n' && ctx.matchesWord("null") == 0 {
		ctx.instructionNewValueNull()
		return false
	}

	ctx.instructionNewValueEnum()
	empty, criticalErr := ctx.parseAndWriteName()
	if criticalErr {
		return criticalErr
	}
	if empty {
		return ctx.err(`unknown value kind, got character: "` + string(ctx.currentC()) + `"`)
	}
	return false
}

func (ctx *parserCtx) parseNumberInputValue() bool {
	ctx.instructionNewValueInt()
	valueTypeAt := len(ctx.res) - 1

	var eof bool
	c := ctx.currentC()
	if c == '-' {
		ctx.charNr++
		ctx.res = append(ctx.res, '-')
		c, eof = ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}
	} else if c == '+' {
		ctx.charNr++
		c, eof = ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}
	}

	// Parse the first set of numbers (the 123 of +123.456e78)
	for {
		if c >= '0' && c <= '9' {
			ctx.res = append(ctx.res, c)
		} else if c == '.' || c == 'e' || c == 'E' {
			break
		} else if c == '_' {
			// Ignore this char
		} else if isPunctuator(c) || ctx.isIgnoredToken(c) || c == ',' {
			// End of number
			return false
		} else {
			return ctx.err(`unexpected character in int or float value, char: "` + string(c) + `"`)
		}

		ctx.charNr++
		c, eof = ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}
	}

	// Parse the numbers behind the comma (the 456 of +123.456e78)
	if c == '.' {
		ctx.res[valueTypeAt] = valueFloat
		ctx.res = append(ctx.res, '.')
		for {
			ctx.charNr++
			c, eof = ctx.checkC(ctx.charNr)
			if eof {
				return ctx.unexpectedEOF()
			}

			if c >= '0' && c <= '9' {
				ctx.res = append(ctx.res, c)
			} else if c == 'e' || c == 'E' {
				break
			} else if c == '_' {
				// Ignore this char
			} else if c == '.' {
				// isPunctuator(c) returns . on this char but from here those are not allowed
				return ctx.err(`unexpected character in float value, char: "` + string(c) + `"`)
			} else if isPunctuator(c) || ctx.isIgnoredToken(c) || c == ',' {
				// End of number
				return false
			} else {
				return ctx.err(`unexpected character in float value, char: "` + string(c) + `"`)
			}
		}
	}

	// Parse the exponent (the 78 of +123.456e78)
	if c == 'e' || c == 'E' {
		ctx.res[valueTypeAt] = valueFloat
		ctx.res = append(ctx.res, 'E')

		ctx.charNr++
		c, eof = ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}
		if c == '+' || c == '-' {
			if c == '-' {
				ctx.res = append(ctx.res, c)
			}
			ctx.charNr++
			c, eof = ctx.checkC(ctx.charNr)
			if eof {
				return ctx.unexpectedEOF()
			}
		}

		for {
			if c >= '0' && c <= '9' {
				ctx.res = append(ctx.res, c)
			} else if c == 'e' || c == 'E' || c == '.' {
				// isPunctuator(c) returns . on this char but from here those are not allowed
				// e and E are also not allowed from here
				return ctx.err(`unexpected character in float value, char: "` + string(c) + `"`)
			} else if c == '_' {
				// Ignore this char
			} else if isPunctuator(c) || ctx.isIgnoredToken(c) || c == ',' {
				// End of number
				break
			} else {
				return ctx.err(`unexpected character in float value, char: "` + string(c) + `"`)
			}

			ctx.charNr++
			c, eof = ctx.checkC(ctx.charNr)
			if eof {
				return ctx.unexpectedEOF()
			}
		}
	}

	return false
}

func (ctx *parserCtx) parseStringInputValue() bool {
	ctx.instructionNewValueString()

	if ctx.matches(`"""`) == 0 {
		// Parse block string
		return ctx.err(`block strings are not supported`)
	}

	// Parse normal string
	for {
		ctx.charNr++
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}

		if c == 0 {
			// TODO add support for this
			continue
		}

		if c == '\n' || c == '\r' {
			return ctx.err("newline and carriage returns not allowed in strings")
		}

		if c == '"' {
			ctx.charNr++
			return false
		}

		if c == '\\' {
			ctx.charNr++
			c, eof = ctx.checkC(ctx.charNr)
			if eof {
				return ctx.unexpectedEOF()
			}

			switch c {
			case 0:
				// TODO add support for this
			case '\n', '\r':
				return ctx.err("newline and carriage returns not allowed in strings")
			case 'b':
				ctx.res = append(ctx.res, '\b')
			case 'f':
				ctx.res = append(ctx.res, '\f')
			case 'n':
				ctx.res = append(ctx.res, '\n')
			case 'r':
				ctx.res = append(ctx.res, '\r')
			case 't':
				ctx.res = append(ctx.res, '\t')
			case 'u':
				ctx.charNr++
				c1, _ := ctx.checkC(ctx.charNr)
				ctx.charNr++
				c2, _ := ctx.checkC(ctx.charNr)
				ctx.charNr++
				c3, _ := ctx.checkC(ctx.charNr)
				ctx.charNr++
				c4, eof := ctx.checkC(ctx.charNr)
				if eof {
					return ctx.unexpectedEOF()
				}

				// we need this 2 times where the largest buffer is required to be 4 bytes
				res := make([]byte, 4)

				_, err := hex.Decode(res, []byte{c1, c2, c3, c4})
				if err != nil {
					return ctx.err(err.Error())
				}
				// if res[0] != 0 {
				// 	ctx.res = append(ctx.res, res[0])
				// }
				// if res[1] != 0 {
				// 	ctx.res = append(ctx.res, res[1])
				// }

				r := utf16.Decode([]uint16{uint16(res[1]) | (uint16(res[0]) << 8)})[0]

				// hex.Decode above only writes to the first and second byte
				res[0] = 0
				res[1] = 0
				l := utf8.EncodeRune(res, r)

				ctx.res = append(ctx.res, res[:l]...)
			default:
				// TODO support unicode
				ctx.res = append(ctx.res, c)
			}
			continue
		}

		ctx.res = append(ctx.res, c)
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

func isPunctuator(c byte) bool {
	return c == '!' || c == '$' || c == '(' || c == ')' || c == '.' || c == ':' || c == '=' || c == '@' || c == '[' || c == ']' || c == '{' || c == '|' || c == '}'
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

func (ctx *parserCtx) matchesWord(oneOf ...string) int {
	startIdx := ctx.charNr

	matches := ctx.matches(oneOf...)
	if matches == -1 {
		return -1
	}
	c, eof := ctx.checkC(ctx.charNr)
	if eof {
		return matches
	}
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
		ctx.charNr = startIdx
		return -1
	}

	return matches
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
				ctx.charNr = startIdx
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

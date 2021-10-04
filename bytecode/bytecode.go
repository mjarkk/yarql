package bytecode

import (
	"encoding/hex"
	"errors"
	"hash"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

type ParserCtx struct {
	Res               []byte
	FragmentLocations []int
	Query             []byte
	charNr            int
	Errors            []error
	target            *string
	hasTarget         bool
	TargetIdx         int // -1 = no matching target was found, >= 0 = res index of target
	Hasher            hash.Hash32
}

func (ctx *ParserCtx) ParseQueryToBytecode(target *string) {
	*ctx = ParserCtx{
		Res:               ctx.Res[:0],
		FragmentLocations: ctx.FragmentLocations[:0],
		Query:             ctx.Query,
		Errors:            ctx.Errors[:0],
		target:            target,
		hasTarget:         target != nil && len(*target) > 0,
		TargetIdx:         -1,
		Hasher:            ctx.Hasher,
	}

	for {
		if ctx.parseOperatorOrFragment() {
			return
		}
	}
}

func (ctx *ParserCtx) writeUint32(value uint32, at int) {
	ctx.Res[at] = byte(0xff & value)
	ctx.Res[at+1] = byte(0xff & (value >> 8))
	ctx.Res[at+2] = byte(0xff & (value >> 16))
	ctx.Res[at+3] = byte(0xff & (value >> 24))
}

// - http://spec.graphql.org/June2018/#sec-Language.Operations
// - http://spec.graphql.org/June2018/#FragmentDefinition
func (ctx *ParserCtx) parseOperatorOrFragment() (stop bool) {
	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return true
	}

	operationStartsAt := len(ctx.Res)
	if c == '{' {
		if !ctx.hasTarget {
			ctx.TargetIdx = operationStartsAt
		}
		ctx.instructionNewOperation(OperatorQuery)
	} else if matches := ctx.matches("query", "mutation", "subscription"); matches != -1 {
		// Set the operation kind
		if !ctx.hasTarget {
			ctx.TargetIdx = operationStartsAt
		}
		if matches == 0 {
			ctx.instructionNewOperation(OperatorQuery)
		} else if matches == 1 {
			ctx.instructionNewOperation(OperatorMutation)
		} else {
			ctx.instructionNewOperation(OperatorSubscription)
		}
		hasArgsFlagLocation := len(ctx.Res) - 2
		directivesCountLocation := len(ctx.Res) - 1

		// Parse operation name
		_, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}

		startOfName := len(ctx.Res)
		_, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}

		name := ctx.Res[startOfName:]
		if len(name) > 0 && ctx.hasTarget && b2s(name) == *ctx.target {
			ctx.TargetIdx = operationStartsAt
		}

		c, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		if c == '(' {
			ctx.Res[hasArgsFlagLocation] = 't'

			// The first byte is to identify the start of the args
			// the other bytes are used to store the end location of the argument
			ctx.Res = append(ctx.Res, 0, 0, 0, 0, 0)
			endAt := len(ctx.Res) - 4
			argumentsStartAt := uint32(len(ctx.Res))

			ctx.charNr++
			criticalErr := ctx.parseOperatorArguments()
			if criticalErr {
				return criticalErr
			}

			argumentsLen := uint32(len(ctx.Res)) - argumentsStartAt
			ctx.writeUint32(argumentsLen, endAt)
		}

		amount, criticalErr := ctx.parseDirectives()
		ctx.Res[directivesCountLocation] = amount
		if criticalErr {
			return criticalErr
		}
		c = ctx.currentC()

		if c != '{' {
			return ctx.err(`expected selection set opener ("{") but got "` + string(c) + `"`)
		}
	} else if matches := ctx.matches("fragment"); matches != -1 {
		ctx.FragmentLocations = append(ctx.FragmentLocations, len(ctx.Res)+1)
		ctx.instructionNewFragment()

		// Parse fragment name
		_, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}
		nameLen, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if nameLen == 0 {
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
		ctx.Res = append(ctx.Res, 0)
		nameLen, criticalErr = ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if nameLen == 0 {
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

func (ctx *ParserCtx) parseOperatorArguments() bool {
	ctx.instructionNewOperationArgs()

	for {
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

		if c == ')' {
			ctx.charNr++
			ctx.instructionEnd()
			return false
		}
		if c != '$' {
			return ctx.err(`expected "$" but got "` + string(c) + `"`)
		}
		ctx.charNr++

		criticalErr := ctx.parseOperatorArgument()
		if criticalErr {
			return criticalErr
		}
	}
}

func (ctx *ParserCtx) parseOperatorArgument() bool {
	// Parse `$` of `query a($some_var: String = "a") {`
	startOfArgument := len(ctx.Res) + 1
	argLengthLocation := ctx.instructionNewOperationArg()

	// Parse `some_name` of `query a($some_var: String = "a") {`
	nameLen, criticalErr := ctx.parseAndWriteName()
	if criticalErr {
		return criticalErr
	}
	if nameLen == 0 {
		return ctx.err(`expected argument name but got "` + string(ctx.currentC()) + `"`)
	}

	// Parse `:` of `query a($some_var: String = "a") {`
	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}
	if c != ':' {
		return ctx.err(`expected ":" name but got "` + string(ctx.currentC()) + `"`)
	}
	ctx.charNr++

	// Parse `String` of `query a($some_var: String = "a") {`
	ctx.Res = append(ctx.Res, 0)
	c, eof = ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}
	criticalErr = ctx.parseGraphqlTypeName(c)
	if criticalErr {
		return criticalErr
	}
	ctx.Res = append(ctx.Res, 0)

	// Parse `=` of query `a($some_var: String = "a") {`
	// If no = is found return and set the has default to false
	c, eof = ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}
	if c == '=' {
		ctx.Res = append(ctx.Res, 't')
		ctx.charNr++

		// Parse `"a"` of `query a($some_var: String = "a") {`
		_, eof = ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}

		criticalErr = ctx.parseInputValue()
		if criticalErr {
			return criticalErr
		}
	} else {
		ctx.Res = append(ctx.Res, 'f')
	}

	endOfArgument := len(ctx.Res)
	ctx.writeUint32(uint32(endOfArgument-startOfArgument), argLengthLocation)
	return false
}

func (ctx *ParserCtx) parseDirectives() (directivesAmount uint8, criticalErr bool) {
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
		hasArgsFlag := len(ctx.Res) - 1
		nameLen, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return directivesAmount, criticalErr
		}
		if nameLen == 0 {
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
		ctx.Res[hasArgsFlag] = 't'
		ctx.charNr++
		criticalErr = ctx.parseAssignmentSet(')')
		if criticalErr {
			return directivesAmount, criticalErr
		}
	}
}

func (ctx *ParserCtx) parseGraphqlTypeName(c byte) bool {
	var eof bool
	operationLocation := len(ctx.Res)

	if c == '[' {
		ctx.Res = append(ctx.Res, 'l')
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
			ctx.Res[operationLocation] = 'L'
			ctx.charNr++
		}

		return false
	}

	ctx.Res = append(ctx.Res, 'n')
	nameLen, criticalErr := ctx.parseAndWriteName()
	if criticalErr {
		return criticalErr
	}
	if nameLen == 0 {
		return ctx.err(`invalid typename char "` + string(ctx.currentC()) + `"`)
	}

	c, eof = ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}
	if c == '!' {
		ctx.Res[operationLocation] = 'N'
		ctx.charNr++
	}

	return false
}

func (ctx *ParserCtx) parseSelectionSet() bool {
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
		directivesCountLocation := len(ctx.Res) - 9
		startField := len(ctx.Res)

		ctx.Res = append(ctx.Res, 0) // write name length
		aliasOrNameLen, criticalError := ctx.parseAndWriteName()
		if criticalError {
			return criticalError
		}
		ctx.Res[startField] = aliasOrNameLen

		if aliasOrNameLen == 0 {
			// Revert changes from ctx.instructionNewField()
			ctx.Res = ctx.Res[:len(ctx.Res)-12]

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
				directivesCountLocation := len(ctx.Res) - 5
				startFragment := len(ctx.Res)

				nameLen, criticalErr := ctx.parseAndWriteName()
				if criticalErr {
					return criticalErr
				}
				c, eof = ctx.mightIgnoreNextTokens()
				if eof {
					return ctx.unexpectedEOF()
				}

				if nameLen == 0 {
					if isInline {
						return ctx.err(`expected fragment type name but got char: "` + string(c) + `"`)
					} else {
						return ctx.err(`expected fragment name but got char: "` + string(c) + `"`)
					}
				}

				if c == '@' {
					amount, criticalErr := ctx.parseDirectives()
					ctx.Res[directivesCountLocation] = amount
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

				ctx.Res = writeUint32At(ctx.Res, startFragment-4, uint32(len(ctx.Res)-startFragment))

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

		nameLenAt := len(ctx.Res)
		ctx.Res = append(ctx.Res, 0)

		if c == ':' {
			ctx.charNr++
			_, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}

			nameLen, criticalErr := ctx.parseAndWriteName()
			if criticalErr {
				return criticalErr
			}
			if nameLen == 0 {
				return ctx.err(`unexpected character, expected valid name char but got "` + string(c) + `"`)
			}
			ctx.Res[nameLenAt] = nameLen

			c, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}

			ctx.Hasher.Reset()
			ctx.Hasher.Write(ctx.Res[nameLenAt+1 : nameLenAt+1+int(nameLen)])
			ctx.Res = writeUint32At(ctx.Res, startField-4, ctx.Hasher.Sum32())
		} else {
			ctx.Hasher.Reset()
			ctx.Hasher.Write(ctx.Res[startField+1 : startField+1+int(aliasOrNameLen)])
			ctx.Res = writeUint32At(ctx.Res, startField-4, ctx.Hasher.Sum32())
		}

		if c == '@' {
			amount, criticalErr := ctx.parseDirectives()
			ctx.Res[directivesCountLocation] = amount
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

			// ctx.charNr++

			c, eof = ctx.mightIgnoreNextTokens()
			if eof {
				return ctx.unexpectedEOF()
			}
		}

		ctx.instructionEnd()
		ctx.writeUint32(uint32(len(ctx.Res)-startField), startField-8)

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
func (ctx *ParserCtx) parseAssignmentSet(closure byte) bool {
	ctx.instructionNewValueObject()
	startOfObj := len(ctx.Res)

	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}
	if c == closure {
		ctx.instructionEnd()
		ctx.charNr++
		ctx.writeUint32(uint32(len(ctx.Res)-startOfObj), startOfObj-4)
		return false
	}

	for {
		ctx.instructionStartNewValueObjectField()

		nameLen, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if nameLen == 0 {
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
			ctx.writeUint32(uint32(len(ctx.Res)-startOfObj), startOfObj-4)
			return false
		}
	}
}

func (ctx *ParserCtx) parseInputValue() bool {
	c, eof := ctx.mightIgnoreNextTokens()
	if eof {
		return ctx.unexpectedEOF()
	}

	if c == '$' {
		ctx.charNr++
		ctx.instructionNewValueVariable()
		startOfVariable := len(ctx.Res)

		nameLen, criticalErr := ctx.parseAndWriteName()
		if criticalErr {
			return criticalErr
		}
		if nameLen == 0 {
			return ctx.err(`variable input should have a name, got character: "` + string(ctx.currentC()) + `"`)
		}

		ctx.writeUint32(uint32(len(ctx.Res)-startOfVariable), startOfVariable-4)
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
		startOfList := len(ctx.Res)

		c, eof := ctx.mightIgnoreNextTokens()
		if eof {
			return ctx.unexpectedEOF()
		}

		if c == ']' {
			ctx.charNr++
			ctx.instructionEnd()
			ctx.writeUint32(uint32(len(ctx.Res)-startOfList), startOfList-4)
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
				ctx.writeUint32(uint32(len(ctx.Res)-startOfList), startOfList-4)
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
	startOfEnum := len(ctx.Res)

	nameLen, criticalErr := ctx.parseAndWriteName()
	if criticalErr {
		return criticalErr
	}
	if nameLen == 0 {
		return ctx.err(`unknown value kind, got character: "` + string(ctx.currentC()) + `"`)
	}

	ctx.writeUint32(uint32(len(ctx.Res)-startOfEnum), startOfEnum-4)
	return false
}

func (ctx *ParserCtx) parseNumberInputValue() bool {
	ctx.instructionNewValueInt()
	startOfInt := len(ctx.Res)

	valueTypeAt := len(ctx.Res) - 5

	var eof bool
	c := ctx.currentC()
	if c == '-' {
		ctx.charNr++
		ctx.Res = append(ctx.Res, '-')
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
			ctx.Res = append(ctx.Res, c)
		} else if c == '.' || c == 'e' || c == 'E' {
			break
		} else if c == '_' {
			// Ignore this char
		} else if isPunctuator(c) || ctx.isIgnoredToken(c) || c == ',' {
			// End of number
			ctx.writeUint32(uint32(len(ctx.Res)-startOfInt), startOfInt-4)
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
		ctx.Res[valueTypeAt] = ValueFloat
		ctx.Res = append(ctx.Res, '.')
		for {
			ctx.charNr++
			c, eof = ctx.checkC(ctx.charNr)
			if eof {
				return ctx.unexpectedEOF()
			}

			if c >= '0' && c <= '9' {
				ctx.Res = append(ctx.Res, c)
			} else if c == 'e' || c == 'E' {
				break
			} else if c == '_' {
				// Ignore this char
			} else if c == '.' {
				// isPunctuator(c) returns . on this char but from here those are not allowed
				return ctx.err(`unexpected character in float value, char: "` + string(c) + `"`)
			} else if isPunctuator(c) || ctx.isIgnoredToken(c) || c == ',' {
				// End of number
				ctx.writeUint32(uint32(len(ctx.Res)-startOfInt), startOfInt-4)
				return false
			} else {
				return ctx.err(`unexpected character in float value, char: "` + string(c) + `"`)
			}
		}
	}

	// Parse the exponent (the 78 of +123.456e78)
	if c == 'e' || c == 'E' {
		ctx.Res[valueTypeAt] = ValueFloat
		ctx.Res = append(ctx.Res, 'E')

		ctx.charNr++
		c, eof = ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}
		if c == '+' || c == '-' {
			if c == '-' {
				ctx.Res = append(ctx.Res, c)
			}
			ctx.charNr++
			c, eof = ctx.checkC(ctx.charNr)
			if eof {
				return ctx.unexpectedEOF()
			}
		}

		for {
			if c >= '0' && c <= '9' {
				ctx.Res = append(ctx.Res, c)
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

	ctx.writeUint32(uint32(len(ctx.Res)-startOfInt), startOfInt-4)
	return false
}

func (ctx *ParserCtx) parseStringInputValue() bool {
	// FIXME block strings are not spec compliant

	ctx.instructionNewValueString()
	startOfString := len(ctx.Res)

	isBlock := ctx.matches(`"""`) == 0
	if isBlock {
		// Trim spaces and enters before text in block string
		for {
			c, eof := ctx.checkC(ctx.charNr)
			if eof {
				return ctx.unexpectedEOF()
			}
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				ctx.charNr++
				continue
			}
			ctx.charNr--
			break
		}
	}

	// Parse normal string
mainLoop:
	for {
		ctx.charNr++
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			return ctx.unexpectedEOF()
		}

		if c == 0 {
			// TODO maybe add support for this
			continue
		}

		if c == '\n' || c == '\r' {
			if !isBlock {
				return ctx.err("newline and carriage returns not allowed in strings")
			}

			ctx.Res = append(ctx.Res, c)
			if c == '\r' {
				c, eof = ctx.checkC(ctx.charNr + 1)
				if !eof && c == '\n' {
					ctx.Res = append(ctx.Res, '\n')
					ctx.charNr++
				}
			}

			for {
				ctx.charNr++
				c, eof := ctx.checkC(ctx.charNr)
				if eof {
					return ctx.unexpectedEOF()
				}
				if c == ' ' || c == '\t' {
					continue
				}
				ctx.charNr--
				continue mainLoop
			}
		}

		if c == '"' {
			if !isBlock {
				ctx.charNr++

				ctx.writeUint32(uint32(len(ctx.Res)-startOfString), startOfString-4)
				return false
			}
			if ctx.matches(`"""`) == 0 {
				// Trim last newlines from the written output
				for {
					lastInst := ctx.Res[len(ctx.Res)-1]
					if lastInst == '\n' || lastInst == '\r' || lastInst == ' ' {
						ctx.Res = ctx.Res[:len(ctx.Res)-1]
						continue
					}
					break
				}

				ctx.writeUint32(uint32(len(ctx.Res)-startOfString), startOfString-4)
				return false
			}
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
				ctx.Res = append(ctx.Res, '\b')
			case 'f':
				ctx.Res = append(ctx.Res, '\f')
			case 'n':
				ctx.Res = append(ctx.Res, '\n')
			case 'r':
				ctx.Res = append(ctx.Res, '\r')
			case 't':
				ctx.Res = append(ctx.Res, '\t')
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

				ctx.Res = append(ctx.Res, res[:l]...)
			default:
				// TODO support unicode
				ctx.Res = append(ctx.Res, c)
			}
			continue
		}

		ctx.Res = append(ctx.Res, c)
	}
}

//
// ITERATOR HELPERS
//

func (ctx *ParserCtx) checkC(nr int) (res byte, end bool) {
	if ctx.eof(nr) {
		return 0, true
	}
	return ctx.c(nr), false
}

func (ctx *ParserCtx) c(nr int) byte {
	return ctx.Query[nr]
}

func (ctx *ParserCtx) eof(nr int) bool {
	return nr >= len(ctx.Query)
}

func (ctx *ParserCtx) currentC() byte {
	return ctx.c(ctx.charNr)
}

func (ctx *ParserCtx) mightIgnoreNextTokens() (nextC byte, eof bool) {
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
func (ctx *ParserCtx) isIgnoredToken(c byte) bool {
	// TODO this doesn't support unicode bomb
	return c == ' ' || c == '\t' || ctx.isLineTerminator() || ctx.isComment(true) || c == 0
}

// https://spec.graphql.org/June2018/#LineTerminator
func (ctx *ParserCtx) isLineTerminator() bool {
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
func (ctx *ParserCtx) isComment(parseComment bool) bool {
	if ctx.currentC() == '#' {
		if parseComment {
			ctx.parseComment()
		}
		return true
	}
	return false
}

func (ctx *ParserCtx) parseComment() {
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

func (ctx *ParserCtx) matchesWord(oneOf ...string) int {
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

func (ctx *ParserCtx) matches(oneOf ...string) int {
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

func (ctx *ParserCtx) err(err string) bool {
	line := uint(1)
	column := uint(0)
	for idx, char := range ctx.Query {
		if idx == ctx.charNr {
			break
		}

		switch char {
		case '\n':
			if column == 0 && idx > 0 && ctx.Query[idx-1] == '\r' {
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

	ctx.Errors = append(ctx.Errors, ErrorWLocation{
		errors.New(err),
		line,
		uint(column),
	})
	return true
}

func (ctx *ParserCtx) unexpectedEOF() bool {
	// panic("DEBUG")
	return ctx.err("unexpected EOF")
}

func (ctx *ParserCtx) parseAndWriteName() (nameLength uint8, criticalError bool) {
	c, eof := ctx.checkC(ctx.charNr)
	if eof {
		return 0, ctx.unexpectedEOF()
	}

	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
		ctx.Res = append(ctx.Res, c)
		ctx.charNr++
		nameLength++
	} else {
		return 0, false
	}

	for {
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			return nameLength, ctx.unexpectedEOF()
		}

		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (c >= '0' && c <= '9') {
			ctx.Res = append(ctx.Res, c)
			ctx.charNr++
			nameLength++
			if nameLength == 255 {
				return nameLength, ctx.err("names cannot be longer than 254 chars")
			}
			continue
		}

		return nameLength, false
	}
}

// b2s converts a byte array into a string without allocating new memory
// Note that any changes to a will result in a diffrent string
func b2s(a []byte) string {
	return *(*string)(unsafe.Pointer(&a))
}

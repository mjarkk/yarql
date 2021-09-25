package graphql

import (
	"errors"

	"github.com/mjarkk/go-graphql/bytecode"
)

type BytecodeCtx struct {
	schema *Schema
	query  bytecode.ParserCtx
	result []byte
	charNr int
}

type BytecodeParseOptions struct {
	NoMeta bool
}

func (ctx *BytecodeCtx) write(b []byte) {
	ctx.result = append(ctx.result, b...)
}

func (ctx *BytecodeCtx) writeByte(b byte) {
	ctx.result = append(ctx.result, b)
}

func (ctx *BytecodeCtx) BytecodeResolve(query []byte, opts BytecodeParseOptions) ([]byte, []error) {
	*ctx = BytecodeCtx{
		schema: ctx.schema,
		query:  ctx.query,
		result: ctx.result[:0],
	}
	ctx.query.Query = append(ctx.query.Query[:0], query...)

	ctx.query.ParseQueryToBytecode()

	if !opts.NoMeta {
		ctx.write([]byte(`{"data":`))
	}

	ctx.resolveOperation()
	ctx.writeByte('}')

	if !opts.NoMeta {
		ctx.writeByte('}')
	}

	return ctx.result, ctx.query.Errors
}

// readInst reads the current instruction and increments the charNr
func (ctx *BytecodeCtx) readInst() byte {
	c := ctx.query.Res[ctx.charNr]
	ctx.charNr++
	return c
}

func (ctx *BytecodeCtx) lastInst() byte {
	return ctx.query.Res[ctx.charNr-1]
}

func (ctx *BytecodeCtx) readBool() bool {
	return ctx.readInst() == 't'
}

func (ctx *BytecodeCtx) err(msg string) bool {
	ctx.query.Errors = append(ctx.query.Errors, errors.New(msg))
	return true
}

func (ctx *BytecodeCtx) resolveOperation() bool {
	ctx.writeByte('{')
	ctx.charNr += 3 // read 0, [ActionOperator], [kind]

	hasArguments := ctx.readBool()
	if hasArguments {
		// TODO
		return ctx.err("arguments currently unsupported")
	}

	directivesCount := ctx.readInst()
	if directivesCount > 0 {
		// TODO
		return ctx.err("operation directives unsupported")
	}

	for {
		// Read name
		if ctx.readInst() == 0 {
			break
		}
	}

	for {
		switch ctx.readInst() {
		case bytecode.ActionEnd:
			// End of operator
			return false
		case bytecode.ActionField:
			// Parse field
			if ctx.resolveField() {
				return true
			}
		default:
			return ctx.err("unsupported operation " + string(ctx.lastInst()))
		}

		criticalErr := ctx.resolveField()
		if criticalErr {
			return criticalErr
		}
	}
}

func (ctx *BytecodeCtx) resolveField() bool {
	// Read directives
	// TODO
	directivesCount := ctx.readInst()
	if directivesCount > 0 {
		return ctx.err("operation directives unsupported")
	}

	// Read field name
	startOfName := ctx.charNr
	endOfName := ctx.charNr
	for {
		if ctx.readInst() == 0 {
			endOfName = ctx.charNr - 1
			break
		}
	}
	name := ctx.query.Res[startOfName:endOfName]

	// TODO
	return ctx.err("field not supported")
}

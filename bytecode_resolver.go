package graphql

import "github.com/mjarkk/go-graphql/bytecode"

type BytecodeCtx struct {
	schema *Schema
	query  bytecode.ParserCtx
	result []byte
}

type BytecodeParseOptions struct {
	ReturnOnlyData bool
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

	if !opts.ReturnOnlyData {
		ctx.write([]byte(`{"data":`))
	}

	ctx.ResolveOp()

	if !opts.ReturnOnlyData {
		ctx.writeByte('}')
	}

	return ctx.result, ctx.query.Errors
}

func (ctx *BytecodeCtx) ResolveOp() {
	// TODO
}

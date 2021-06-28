package graphql

func (i *iterT) ParseQueryAndCheckNames(input string, ctx *Ctx) {
	if ctx != nil {
		ctx.startTrace()
	}

	i.parseQuery(input)
	if len(i.resErrors) > 0 {
		return
	}
	if ctx != nil {
		ctx.finishTrace(func(offset, duration int64) {
			ctx.tracing.Parsing.StartOffset = offset
			ctx.tracing.Parsing.Duration = duration
		})

		// TODO minimize the code below
		ctx.startTrace()
		ctx.finishTrace(func(offset, duration int64) {
			ctx.tracing.Validation.StartOffset = offset
			ctx.tracing.Validation.Duration = duration
		})
	}
}

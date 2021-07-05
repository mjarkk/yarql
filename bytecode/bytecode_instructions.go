package graphql

type action = byte

const (
	actionEnd      action = 'e'
	actionOperator action = 'o'
	actionField    action = 'f'
)

type operatorKind = byte

const (
	operatorQuery        operatorKind = 'q'
	operatorMutation     operatorKind = 'm'
	operatorSubscription operatorKind = 's'
)

// represends:
//
// query {
// ^- Kind
//
// writes:
// i [actionNewOperator] [kind]
//
// additional append:
// [name...]
func (ctx *parserCtx) instructionNewOperation(kind operatorKind) int {
	res := len(ctx.res)
	ctx.res = append(ctx.res, 0, actionOperator, kind)
	return res
}

// represends:
//
// query { a }
//         ^
//
// writes:
// i [actionField]
func (ctx *parserCtx) instructionNewField() {
	ctx.res = append(ctx.res, 0, actionField)
}

// represends:
//
// query { }
//         ^- End
//
// query { a { } }
//             ^- End
//
// query { a  }
//          ^- End
//
// writes:
// i [actionEndClosure]
func (ctx *parserCtx) instructionEnd() {
	ctx.res = append(ctx.res, 0, actionEnd)
}

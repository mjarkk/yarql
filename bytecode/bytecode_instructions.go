package graphql

type action = byte

const (
	actionEndClosure action = iota + 1
	actionNewOperator
)

type operatorKind = byte

const (
	operatorQuery operatorKind = iota + 1
	operatorMutation
	operatorSubscription
)

// represends:
//
// query {
// ^- Kind
//
// writes:
// 0 [actionNewOperator] [kind]
//
// additional append:
// [name...]
func (ctx *parserCtx) instructionNewOperation(kind operatorKind) int {
	res := len(ctx.res)
	ctx.res = append(ctx.res, 0, actionNewOperator, kind)
	return res
}

// represends:
//
// query { }
//         ^- Closure
//
// query { a { } }
//             ^- Closure
//
func (ctx *parserCtx) instructionEndClosure() {
	ctx.res = append(ctx.res, 0, actionEndClosure)
}

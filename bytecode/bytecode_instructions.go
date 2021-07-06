package graphql

type action = byte

const (
	actionEnd      action = 'e'
	actionOperator action = 'o'
	actionField    action = 'f'
	actionSpread   action = 's'
	actionFragment action = 'F'
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
// 0 [actionNewOperator] [kind]
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
// fragment InputValue on __InputValue {
//          ^- Name       ^- Type Name
//
// writes:
// 0 [actionFragment]
//
// additional required append:
// [Name] 0 [Type Name]
func (ctx *parserCtx) instructionNewFragment() int {
	res := len(ctx.res)
	ctx.res = append(ctx.res, 0, actionFragment)
	return res
}

// represends:
//
// query { a }
//         ^
//
// writes:
// 0 [actionField]
func (ctx *parserCtx) instructionNewField() {
	ctx.res = append(ctx.res, 0, actionField)
}

// represends:
//
// {
//   ...Foo
//   ^- Fragment spread in selector
//   ... on Banana {}
//   ^- Also a fragment spread
// }
//
// writes:
// 0 [actionSpread] [t/f (t = inline fragment, f = pointer to fragment)]
//
// additional required append:
// [Typename or Fragment Name]
func (ctx *parserCtx) instructionNewFragmentSpread(isInline bool) {
	if isInline {
		ctx.res = append(ctx.res, 0, actionSpread, 't')
	} else {
		ctx.res = append(ctx.res, 0, actionSpread, 'f')
	}
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
// 0 [actionEndClosure]
func (ctx *parserCtx) instructionEnd() {
	ctx.res = append(ctx.res, 0, actionEnd)
}

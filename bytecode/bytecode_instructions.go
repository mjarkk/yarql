package graphql

type action = byte

const (
	actionEnd              action = 'e'
	actionOperator         action = 'o'
	actionField            action = 'f'
	actionSpread           action = 's'
	actionFragment         action = 'F'
	actionValue            action = 'v'
	actionObjectValueField action = 'u'
)

type valueKind = byte

const (
	valueVariable valueKind = 'v'
	valueInt      valueKind = 'i'
	valueFloat    valueKind = 'f'
	valueString   valueKind = 's'
	valueBoolean  valueKind = 'b'
	valueNull     valueKind = 'n'
	valueEnum     valueKind = 'e'
	valueList     valueKind = 'l'
	valueObject   valueKind = 'o'
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
//
// additional required append:
// [Fieldname] 0
// OR
// [Alias] 0 [Fieldname]
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
// {a: "a", b: "b", ...}
// ^- This represends the start of a set
// AND
// (a: "a", b: "b", ...)
// ^- This represends the start of a set
//
// writes:
// 0 [actionValue] [valueObject]
func (ctx *parserCtx) instructionNewValueObject() {
	ctx.res = append(ctx.res, 0, actionValue, valueObject)
}

func (ctx *parserCtx) instructionNewValueBoolean(val bool) {
	if val {
		ctx.res = append(ctx.res, 0, actionValue, valueBoolean, '1')
	} else {
		ctx.res = append(ctx.res, 0, actionValue, valueBoolean, '0')
	}
}

func (ctx *parserCtx) instructionNewValueNull() {
	ctx.res = append(ctx.res, 0, actionValue, valueNull)
}

// represends:
//
// {a: "a", b: "b", ...}
//          ^- This represends a field inside a set
// AND
// (a: "a", b: "b", ...)
//          ^- This represends a field inside a set
//
// writes:
// 0 [actionObjectValueField]
//
// additional required append:
// [fieldname]
func (ctx *parserCtx) instructionStartNewValueObjectField() {
	ctx.res = append(ctx.res, 0, actionObjectValueField)
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

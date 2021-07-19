package bytecode

type action = byte

const (
	actionEnd              action = 'e'
	actionOperator         action = 'o'
	actionOperatorArgs     action = 'A'
	actionOperatorArg      action = 'a'
	actionField            action = 'f'
	actionSpread           action = 's'
	actionFragment         action = 'F'
	actionValue            action = 'v'
	actionObjectValueField action = 'u'
	actionDirective        action = 'd'
)

type valueKind = byte

const (
	valueVariable valueKind = '$'
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
// 0 [actionNewOperator] [kind] [f (has no arguments (t = has arguments))] [nr of directives in uint8]
//
// additional append:
// [name...]
func (ctx *ParserCtx) instructionNewOperation(kind operatorKind) int {
	res := len(ctx.Res)
	ctx.Res = append(ctx.Res, 0, actionOperator, kind, 'f', 0)
	return res
}

func (ctx *ParserCtx) instructionNewOperationArgs() {
	ctx.Res = append(ctx.Res, 0, actionOperatorArgs)
}

// represends:
//
// query foo(banana: String) {
//             ^- actionOperatorArg
//
// writes:
// 0 [actionOperatorArg]
//
// additional required append:
// [Name] 0 [Graphql Type] 0 ['t'/'f' (t = has default value (next instruction will value), f = no default value)]
func (ctx *ParserCtx) instructionNewOperationArg() {
	ctx.Res = append(ctx.Res, 0, actionOperatorArg)
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
func (ctx *ParserCtx) instructionNewFragment() int {
	res := len(ctx.Res)
	ctx.Res = append(ctx.Res, 0, actionFragment)
	return res
}

// represends:
//
// query { a }
//         ^
//
// writes:
// 0 [actionField] [directives count as uint8]
//
// additional required append:
// [Fieldname] 0
// OR
// [Alias] 0 [Fieldname]
func (ctx *ParserCtx) instructionNewField() {
	ctx.Res = append(ctx.Res, 0, actionField, 0)
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
// 0 [actionSpread] [t/f (t = inline fragment, f = pointer to fragment)] [directives count in uint8]
//
// additional required append:
// [Typename or Fragment Name]
func (ctx *ParserCtx) instructionNewFragmentSpread(isInline bool) {
	if isInline {
		ctx.Res = append(ctx.Res, 0, actionSpread, 't', 0)
	} else {
		ctx.Res = append(ctx.Res, 0, actionSpread, 'f', 0)
	}
}

// represends:
//
// @banana(arg: 2)
//
// writes:
// 0 [actionDirective] [t/f (t = has arguments, f = no arguments)]
//
// additional required append:
// [Directive name]
func (ctx *ParserCtx) instructionNewDirective() {
	ctx.Res = append(ctx.Res, 0, actionDirective, 'f')
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
func (ctx *ParserCtx) instructionNewValueObject() {
	ctx.Res = append(ctx.Res, 0, actionValue, valueObject)
}

func (ctx *ParserCtx) instructionNewValueList() {
	ctx.Res = append(ctx.Res, 0, actionValue, valueList)
}

func (ctx *ParserCtx) instructionNewValueBoolean(val bool) {
	if val {
		ctx.Res = append(ctx.Res, 0, actionValue, valueBoolean, '1')
	} else {
		ctx.Res = append(ctx.Res, 0, actionValue, valueBoolean, '0')
	}
}

func (ctx *ParserCtx) instructionNewValueNull() {
	ctx.Res = append(ctx.Res, 0, actionValue, valueNull)
}

func (ctx *ParserCtx) instructionNewValueEnum() {
	ctx.Res = append(ctx.Res, 0, actionValue, valueEnum)
}

func (ctx *ParserCtx) instructionNewValueVariable() {
	ctx.Res = append(ctx.Res, 0, actionValue, valueVariable)
}

func (ctx *ParserCtx) instructionNewValueInt() {
	ctx.Res = append(ctx.Res, 0, actionValue, valueInt)
}

func (ctx *ParserCtx) instructionNewValueString() {
	ctx.Res = append(ctx.Res, 0, actionValue, valueString)
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
func (ctx *ParserCtx) instructionStartNewValueObjectField() {
	ctx.Res = append(ctx.Res, 0, actionObjectValueField)
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
func (ctx *ParserCtx) instructionEnd() {
	ctx.Res = append(ctx.Res, 0, actionEnd)
}

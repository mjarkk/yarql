package bytecode

type Action = byte

const (
	ActionEnd              Action = 'e'
	ActionOperator         Action = 'o'
	ActionOperatorArgs     Action = 'A'
	ActionOperatorArg      Action = 'a'
	ActionField            Action = 'f'
	ActionSpread           Action = 's'
	ActionFragment         Action = 'F'
	ActionValue            Action = 'v'
	ActionObjectValueField Action = 'u'
	ActionDirective        Action = 'd'
)

type ValueKind = byte

const (
	ValueVariable ValueKind = '$'
	ValueInt      ValueKind = 'i'
	ValueFloat    ValueKind = 'f'
	ValueString   ValueKind = 's'
	ValueBoolean  ValueKind = 'b'
	ValueNull     ValueKind = 'n'
	ValueEnum     ValueKind = 'e'
	ValueList     ValueKind = 'l'
	ValueObject   ValueKind = 'o'
)

type OperatorKind = byte

const (
	OperatorQuery        OperatorKind = 'q'
	OperatorMutation     OperatorKind = 'm'
	OperatorSubscription OperatorKind = 's'
)

// represends:
//
// query {
// ^- Kind
//
// writes:
// 0 [actionNewOperator] [kind] [t/f (has arguments)] [nr of directives in uint8]
//
// additional append:
// [name...]
func (ctx *ParserCtx) instructionNewOperation(kind OperatorKind) int {
	res := len(ctx.Res)
	ctx.Res = append(ctx.Res, 0, ActionOperator, kind, 'f', 0)
	return res
}

func (ctx *ParserCtx) instructionNewOperationArgs() {
	ctx.Res = append(ctx.Res, 0, ActionOperatorArgs)
}

// represends:
//
// query foo(banana: String) {
//             ^- ActionOperatorArg
//
// writes:
// 0 [ActionOperatorArg] [0000 (encoded uint32 telling how long this full instruction is)]
//
// additional required append:
// [Name] 0 [Graphql Type] 0 [t/f (has a default value?)]
//
// returns:
// the start location of the 4 bit encoded uint32
func (ctx *ParserCtx) instructionNewOperationArg() int {
	ctx.Res = append(ctx.Res, 0, ActionOperatorArg, 0, 0, 0, 0)
	return len(ctx.Res) - 4
}

// represends:
//
// fragment InputValue on __InputValue {
//          ^- Name       ^- Type Name
//
// writes:
// 0 [ActionFragment]
//
// additional required append:
// [Name] 0 [Type Name]
func (ctx *ParserCtx) instructionNewFragment() int {
	res := len(ctx.Res)
	ctx.Res = append(ctx.Res, 0, ActionFragment)
	return res
}

// represends:
//
// query { a }
//         ^
//
// writes:
// 0 [actionField] [directives count as uint8] [0000 length remainder of field]
//
// additional required append:
// [Fieldname] 0
// OR
// [Alias] 0 [Fieldname]
func (ctx *ParserCtx) instructionNewField() {
	ctx.Res = append(ctx.Res, 0, ActionField, 0, 0, 0, 0, 0)
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
// 0 [ActionSpread] [t/f (t = inline fragment, f = pointer to fragment)] [directives count in uint8]
//
// additional required append:
// [Typename or Fragment Name]
func (ctx *ParserCtx) instructionNewFragmentSpread(isInline bool) {
	if isInline {
		ctx.Res = append(ctx.Res, 0, ActionSpread, 't', 0)
	} else {
		ctx.Res = append(ctx.Res, 0, ActionSpread, 'f', 0)
	}
}

// represends:
//
// @banana(arg: 2)
//
// writes:
// 0 [ActionDirective] [t/f (t = has arguments, f = no arguments)]
//
// additional required append:
// [Directive name]
func (ctx *ParserCtx) instructionNewDirective() {
	ctx.Res = append(ctx.Res, 0, ActionDirective, 'f')
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
// 0 [ActionValue] [ValueObject]
func (ctx *ParserCtx) instructionNewValueObject() {
	ctx.Res = append(ctx.Res, 0, ActionValue, ValueObject, 0, 0, 0, 0)
}

func (ctx *ParserCtx) instructionNewValueList() {
	ctx.Res = append(ctx.Res, 0, ActionValue, ValueList, 0, 0, 0, 0)
}

func (ctx *ParserCtx) instructionNewValueBoolean(val bool) {
	if val {
		ctx.Res = append(ctx.Res, 0, ActionValue, ValueBoolean, 1, 0, 0, 0, '1')
	} else {
		ctx.Res = append(ctx.Res, 0, ActionValue, ValueBoolean, 1, 0, 0, 0, '0')
	}
}

func (ctx *ParserCtx) instructionNewValueNull() {
	ctx.Res = append(ctx.Res, 0, ActionValue, ValueNull, 0, 0, 0, 0)
}

// writes:
// 0 [ActionValue] [ActionValue...] [0000 the number of bytes the value will take up encoded as uint32]
func (ctx *ParserCtx) instructionNewValueEnum() {
	ctx.Res = append(ctx.Res, 0, ActionValue, ValueEnum, 0, 0, 0, 0)
}

// writes:
// 0 [ActionValue] [ValueVariable...] [0000 the number of bytes the value will take up encoded as uint32]
func (ctx *ParserCtx) instructionNewValueVariable() {
	ctx.Res = append(ctx.Res, 0, ActionValue, ValueVariable, 0, 0, 0, 0)
}

// writes:
// 0 [ActionValue] [valueInt] [0000 the number of bytes the value will take up encoded as uint32]
func (ctx *ParserCtx) instructionNewValueInt() {
	ctx.Res = append(ctx.Res, 0, ActionValue, ValueInt, 0, 0, 0, 0)
}

// writes:
// 0 [ActionValue] [valueString...] [0000 the number of bytes the value will take up encoded as uint32]
func (ctx *ParserCtx) instructionNewValueString() {
	ctx.Res = append(ctx.Res, 0, ActionValue, ValueString, 0, 0, 0, 0)
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
// 0 [ActionObjectValueField]
//
// additional required append:
// [fieldname]
func (ctx *ParserCtx) instructionStartNewValueObjectField() {
	ctx.Res = append(ctx.Res, 0, ActionObjectValueField)
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
// 0 [ActionEndClosure]
func (ctx *ParserCtx) instructionEnd() {
	ctx.Res = append(ctx.Res, 0, ActionEnd)
}

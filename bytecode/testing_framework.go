package bytecode

import (
	"strconv"
)

func writeUint32At(res []byte, at int, value uint32) []byte {
	res[at] = byte(0xff & value)
	res[at+1] = byte(0xff & (value >> 8))
	res[at+2] = byte(0xff & (value >> 16))
	res[at+3] = byte(0xff & (value >> 24))
	return res
}

type testOperator struct {
	kind       OperatorKind // default = OperatorQuery
	name       string
	args       []testOperatorArg
	directives []testDirective
	fields     []testField
}

func (o testOperator) toBytes() []byte {
	if o.kind == 0 {
		o.kind = OperatorQuery
	}
	res := []byte{0, 'o', o.kind}
	if len(o.args) > 0 { // has args
		res = append(res, 't')
	} else {
		res = append(res, 'f')
	}
	res = append(res, byte(len(o.directives))) // directs count
	res = append(res, []byte(o.name)...)
	if len(o.args) > 0 {
		res = append(res, 0, 0, 0, 0, 0) // length of args
		start := len(res)
		res = append(res, 0, 'A') // args
		for _, arg := range o.args {
			res = arg.toBytes(res)
		}
		res = append(res, 0, 'e') // end of args
		end := len(res)
		res = writeUint32At(res, start-4, uint32(end-start))
	}
	if len(o.directives) > 0 {
		for _, directive := range o.directives {
			res = directive.toBytes(res)
		}
	}
	if len(o.fields) > 0 {
		for _, field := range o.fields {
			res = field.toBytes(res)
		}
	}
	res = append(res, 0, 'e') // end of query
	return res
}

type testFragment struct {
	name   string // REQUIRED
	on     string // REQUIRED
	fields []testField
}

func (o testFragment) toBytes() []byte {
	res := []byte{0, 'F'}
	res = append(res, []byte(o.name)...)
	res = append(res, 0)
	res = append(res, []byte(o.on)...)
	if len(o.fields) > 0 {
		for _, field := range o.fields {
			res = field.toBytes(res)
		}
	}
	res = append(res, 0, 'e')
	return res
}

type testDirective struct {
	name      string // REQUIRED
	arguments []typeObjectValue
}

func (o testDirective) toBytes(res []byte) []byte {
	res = append(res, 0, 'd')
	if o.arguments != nil {
		res = append(res, 't')
	} else {
		res = append(res, 'f')
	}
	res = append(res, []byte(o.name)...)
	if o.arguments != nil {
		res = testValue{
			kind:        ValueObject,
			objectValue: o.arguments,
		}.toBytes(res)
	}
	return res
}

type testOperatorArg struct {
	name         string // REQUIRED
	type_        string // REQUIRED
	defaultValue *testValue
}

func (o testOperatorArg) toBytes(res []byte) []byte {
	start := len(res) + 1
	res = append(res, 0, 'a')     // arg instruction
	res = append(res, 0, 0, 0, 0) // arg length
	res = append(res, []byte(o.name)...)
	res = append(res, 0)
	res = append(res, []byte(o.type_)...)
	if o.defaultValue != nil { // has deafult value
		res = append(res, 0, 't')
		res = o.defaultValue.toBytes(res)
	} else {
		res = append(res, 0, 'f')
	}
	end := len(res)

	res = writeUint32At(res, start+1, uint32(end-start))

	return res
}

type testValue struct {
	kind ValueKind

	// ValueList:
	list []testValue

	// ValueInt
	intValue int64

	// ValueFloat
	floatValue string

	// stringValue
	stringValue string

	// ValueBoolean
	boolValue bool

	// ValueObject
	objectValue []typeObjectValue

	// ValueVariable
	variableValue string

	// ValueEnum
	enumValue string
}

type typeObjectValue struct {
	name  string
	value testValue
}

func (o testValue) toBytes(res []byte) []byte {
	res = append(res, 0, 'v', o.kind) // value instruction
	res = append(res, 0, 0, 0, 0)
	start := len(res)

	switch o.kind {
	case ValueVariable:
		res = append(res, []byte(o.variableValue)...)
	case ValueInt:
		res = strconv.AppendInt(res, o.intValue, 10)
	case ValueFloat:
		res = append(res, []byte(o.floatValue)...)
	case ValueString:
		res = append(res, []byte(o.stringValue)...)
	case ValueBoolean:
		if o.boolValue {
			res = append(res, '1')
		} else {
			res = append(res, '0')
		}
	case ValueNull:
		// Do nothing :)
	case ValueEnum:
		res = append(res, []byte(o.enumValue)...)
	case ValueList:
		for _, item := range o.list {
			res = item.toBytes(res)
		}
		res = append(res, 0, 'e')
	case ValueObject:
		for _, entry := range o.objectValue {
			res = append(res, 0, 'u')
			res = append(res, []byte(entry.name)...)
			res = entry.value.toBytes(res)
		}
		res = append(res, 0, 'e')
	}

	end := len(res)
	res = writeUint32At(res, start-4, uint32(end-start))

	return res
}

type testField struct {
	name       string // REQUIRED
	fields     []testField
	isFragment bool
	directives []testDirective

	// only if isFragment == false
	alias     string
	arguments []typeObjectValue
}

func (o testField) toBytes(res []byte) []byte {
	if o.isFragment {
		res = append(res, 0, 's') // start of fragment
		if o.fields != nil {
			res = append(res, 't')
		} else {
			res = append(res, 'f')
		}
		res = append(res, byte(len(o.directives)))
		res = append(res, []byte(o.name)...)
		for _, directive := range o.directives {
			res = directive.toBytes(res)
		}
		if o.fields != nil {
			for _, field := range o.fields {
				res = field.toBytes(res)
			}
			res = append(res, 0, 'e')
		}
		return res
	}

	res = append(res, 0, 'f')
	res = append(res, byte(len(o.directives)))
	res = append(res, 0, 0, 0, 0)
	start := len(res)

	res = append(res, byte(len(o.name)))
	res = append(res, []byte(o.name)...)
	res = append(res, byte(len(o.alias)))
	res = append(res, []byte(o.alias)...)
	for _, directive := range o.directives {
		res = directive.toBytes(res)
	}
	for _, field := range o.fields {
		res = field.toBytes(res)
	}
	if o.arguments != nil {
		res = testValue{
			kind:        ValueObject,
			objectValue: o.arguments,
		}.toBytes(res)
	}
	res = append(res, 0, 'e')

	end := len(res)
	res = writeUint32At(res, start-4, uint32(end-start))

	return res
}

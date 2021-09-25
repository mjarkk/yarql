package graphql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/mjarkk/go-graphql/bytecode"
)

type BytecodeCtx struct {
	// private
	schema  *Schema
	query   bytecode.ParserCtx
	charNr  int
	context context.Context
	// path    []byte // TODO

	// Zero alloc values
	result                 []byte
	reflectValues          [256]reflect.Value
	currentReflectValueIdx uint8
	funcInputs             []reflect.Value

	// public / kinda public fields
	values map[string]interface{} // API User values, user can put all their shitty things in here like poems or tax papers

}

func (ctx *BytecodeCtx) getGoValue() reflect.Value {
	return ctx.reflectValues[ctx.currentReflectValueIdx]
}

func (ctx *BytecodeCtx) setNextGoValue(value reflect.Value) {
	ctx.currentReflectValueIdx++
	ctx.setGoValue(value)
}

func (ctx *BytecodeCtx) setGoValue(value reflect.Value) {
	ctx.reflectValues[ctx.currentReflectValueIdx] = value
}

type BytecodeParseOptions struct {
	NoMeta         bool            // Returns only the data
	Context        context.Context // Request context
	OperatorTarget string
	Values         map[string]interface{} // Passed directly to the request context

	// TODO support:
	// Variables      string // Expects valid JSON or empty string
	// GetFormFile    func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
	// Tracing        bool                                            // https://github.com/apollographql/apollo-tracing
}

func (ctx *BytecodeCtx) GetValue(key string) (value interface{}) {
	if ctx.values == nil {
		return nil
	}
	return ctx.values[key]
}
func (ctx *BytecodeCtx) GetValueOk(key string) (value interface{}, found bool) {
	if ctx.values == nil {
		return nil, false
	}
	val, ok := ctx.values[key]
	return val, ok
}
func (ctx *BytecodeCtx) SetValue(key string, value interface{}) {
	if ctx.values == nil {
		ctx.values = map[string]interface{}{
			key: value,
		}
	} else {
		ctx.values[key] = value
	}
}

func (ctx *BytecodeCtx) write(b []byte) {
	ctx.result = append(ctx.result, b...)
}

func (ctx *BytecodeCtx) writeByte(b byte) {
	ctx.result = append(ctx.result, b)
}

func (ctx *BytecodeCtx) writeQouted(b []byte) {
	ctx.writeByte('"')
	ctx.write(b)
	ctx.writeByte('"')
}

var nullBytes = []byte("null")

func (ctx *BytecodeCtx) writeNull() {
	ctx.write(nullBytes)
}

func (ctx *BytecodeCtx) BytecodeResolve(query []byte, opts BytecodeParseOptions) ([]byte, []error) {
	*ctx = BytecodeCtx{
		schema:  ctx.schema,
		query:   ctx.query,
		charNr:  0,
		context: opts.Context,

		result:                 ctx.result[:0],
		reflectValues:          ctx.reflectValues,
		currentReflectValueIdx: 0,
		funcInputs:             ctx.funcInputs,

		values: opts.Values,
	}
	ctx.query.Query = append(ctx.query.Query[:0], query...)

	if len(opts.OperatorTarget) > 0 {
		ctx.query.ParseQueryToBytecode(&opts.OperatorTarget)
	} else {
		ctx.query.ParseQueryToBytecode(nil)
	}

	if !opts.NoMeta {
		ctx.write([]byte(`{"data":`))
	}

	ctx.charNr = ctx.query.TargetIdx
	if ctx.charNr == -1 {
		ctx.write([]byte("{}"))
		if len(opts.OperatorTarget) > 0 {
			ctx.err("no operator with name " + opts.OperatorTarget + " found")
		} else {
			ctx.err("no operator found")
		}
	} else {
		ctx.writeByte('{')
		ctx.resolveOperation()
		ctx.writeByte('}')
	}

	if !opts.NoMeta {
		// TODO write remainer of meta to output
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

func (ctx *BytecodeCtx) seekInst() byte {
	return ctx.query.Res[ctx.charNr]
}

func (ctx *BytecodeCtx) skipInst(num int) {
	ctx.charNr += num
}

func (ctx *BytecodeCtx) lastInst() byte {
	return ctx.query.Res[ctx.charNr-1]
}

func (ctx *BytecodeCtx) readBool() bool {
	return ctx.readInst() == 't'
}

func (ctx *BytecodeCtx) err(msg string) bool {
	// TODO support path
	ctx.query.Errors = append(ctx.query.Errors, errors.New(msg))
	return true
}

func (ctx *BytecodeCtx) errf(msg string, args ...interface{}) bool {
	ctx.query.Errors = append(ctx.query.Errors, fmt.Errorf(msg, args...))
	return true
}

func (ctx *BytecodeCtx) resolveOperation() bool {
	ctx.charNr += 2 // read 0, [ActionOperator], [kind]

	kind := ctx.readInst()
	switch kind {
	case bytecode.OperatorQuery:
		ctx.reflectValues[0] = ctx.schema.rootQueryValue
	case bytecode.OperatorMutation:
		ctx.reflectValues[0] = ctx.schema.rootMethodValue
	case bytecode.OperatorSubscription:
		return ctx.err("subscriptions are not supported")
	}

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

	return ctx.resolveSelectionSet(ctx.schema.rootQuery, 0)
}

func (ctx *BytecodeCtx) resolveSelectionSet(typeObj *obj, dept uint8) bool {
	dept++

	firstField := true
	for {
		switch ctx.readInst() {
		case bytecode.ActionEnd:
			// End of operator
			return false
		case bytecode.ActionField:
			// Parse field
			criticalErr := ctx.resolveField(typeObj, dept, !firstField)
			if criticalErr {
				return criticalErr
			}
			firstField = false
		default:
			return ctx.err("unsupported operation " + string(ctx.lastInst()))
		}
	}
}

func (ctx *BytecodeCtx) resolveField(typeObj *obj, dept uint8, addCommaBefore bool) bool {
	// Read directives
	// TODO
	directivesCount := ctx.readInst()
	if directivesCount > 0 {
		return ctx.err("operation directives unsupported")
	}

	// Read field name/alias
	startOfAlias := ctx.charNr
	endOfAlias := ctx.charNr
	for {
		if ctx.readInst() == 0 {
			endOfAlias = ctx.charNr - 1
			break
		}
	}
	alias := ctx.query.Res[startOfAlias:endOfAlias]

	startOfName := startOfAlias
	endOfName := endOfAlias

	// If alias is used read the name
	if ctx.readInst() != 0 {
		startOfName = ctx.charNr - 1
		for {
			if ctx.seekInst() == 0 {
				endOfName = ctx.charNr
				ctx.charNr++
				break
			}
			ctx.charNr++
		}
	}

	name := ctx.query.Res[startOfName:endOfName]
	nameStr := b2s(name)

	if addCommaBefore {
		ctx.writeByte(',')
	}

	ctx.writeQouted(alias)
	ctx.writeByte(':')

	typeObjField, ok := typeObj.objContents[nameStr]
	if !ok {
		ctx.writeNull()
		ctx.errf("%s does not exists on %s", nameStr, typeObj.typeName)
		return false
	}

	if typeObjField.customObjValue != nil {
		ctx.setNextGoValue(*typeObjField.customObjValue)
	} else {
		goValue := ctx.getGoValue()
		if typeObjField.valueType == valueTypeMethod && typeObjField.method.isTypeMethod {
			ctx.setNextGoValue(goValue.Method(typeObjField.structFieldIdx))
		} else {
			ctx.setNextGoValue(goValue.Field(typeObjField.structFieldIdx))
		}
	}

	fieldHasSelection := ctx.seekInst() != 'e'
	criticalErr := ctx.resolveFieldDataValue(typeObjField, dept, fieldHasSelection)
	ctx.currentReflectValueIdx--

	inst := ctx.readInst()
	if inst == bytecode.ActionEnd {
		if ctx.readInst() != 0 {
			return ctx.errf("internal parsing error #2, %v", ctx.lastInst())
		}
	} else if inst == 0 {
		// the 'e' is already parsed by resolveSelectionSet
	} else {
		return ctx.errf("internal parsing error #1, %v", ctx.lastInst())
	}
	return criticalErr
}

func (ctx *BytecodeCtx) resolveFieldDataValue(typeObj *obj, dept uint8, hasSubSelection bool) bool {
	goValue := ctx.getGoValue()

	switch typeObj.valueType {
	case valueTypeUndefined:
		ctx.writeNull()
	case valueTypeArray:
		if (goValue.Kind() != reflect.Array && goValue.Kind() != reflect.Slice) || goValue.IsNil() {
			ctx.writeNull()
			return false
		}

		if typeObj.innerContent == nil {
			ctx.writeNull()
			return ctx.err("internal parsing error #3")
		}
		typeObj = typeObj.innerContent

		ctx.writeByte('[')
		ctx.currentReflectValueIdx++
		goValueLen := goValue.Len()

		startCharNr := ctx.charNr
		for i := 0; i < goValueLen; i++ {
			ctx.charNr = startCharNr

			// prefPathLen := len(ctx.path)
			// ctx.path = append(ctx.path, ',')
			// ctx.path = strconv.AppendInt(ctx.path, int64(i), 10)

			ctx.setGoValue(goValue.Index(i))

			ctx.resolveFieldDataValue(typeObj, dept, hasSubSelection)
			if i < goValueLen-1 {
				ctx.writeByte(',')
			}

			// ctx.path = ctx.path[:prefPathLen]
		}
		ctx.currentReflectValueIdx--
		ctx.writeByte(']')
	case valueTypeObj, valueTypeObjRef:
		if !hasSubSelection {
			ctx.writeNull()
			return ctx.err("must have a selection")
		}

		var ok bool
		if typeObj.valueType == valueTypeObjRef {
			typeObj, ok = ctx.schema.types[typeObj.typeName]
			if !ok {
				ctx.writeNull()
				return false
			}
		}

		ctx.writeByte('{')
		criticalErr := ctx.resolveSelectionSet(typeObj, dept)
		ctx.writeByte('}')
		return criticalErr
	case valueTypeData:
		if hasSubSelection {
			ctx.writeNull()
			return ctx.err("cannot have a selection set on this field")
		}

		if typeObj.isID && typeObj.dataValueType != reflect.String {
			// Graphql ID fields are always strings
			ctx.writeByte('"')
			ctx.valueToJson(goValue, typeObj.dataValueType)
			ctx.writeByte('"')
		} else {
			ctx.valueToJson(goValue, typeObj.dataValueType)
		}
	case valueTypePtr:
		if goValue.Kind() != reflect.Ptr || goValue.IsNil() {
			ctx.writeNull()
		} else {
			ctx.reflectValues[ctx.currentReflectValueIdx] = goValue.Elem()
			return ctx.resolveFieldDataValue(typeObj, dept, hasSubSelection)
		}
	case valueTypeMethod:
		method := typeObj.method

		if !method.isTypeMethod && goValue.IsNil() {
			ctx.writeNull()
			return false
		}

		ctx.funcInputs = ctx.funcInputs[:0]
		for _, in := range method.ins {
			if in.isCtx {
				// TODO THIS IS A DIFFRENT CONTEXT AND WILL PANIC
				ctx.funcInputs = append(ctx.funcInputs, ctx.schema.ctxReflection)
				fmt.Println("context argument currently unsupported")
				return ctx.err("internal server error #4")
			} else {
				ctx.funcInputs = append(ctx.funcInputs, reflect.New(*in.type_).Elem())
			}
		}

		// TODO parse arguments

		outs := goValue.Call(ctx.funcInputs)
		if method.errorOutNr != nil {
			errOut := outs[*method.errorOutNr]
			if !errOut.IsNil() {
				err, ok := errOut.Interface().(error)
				if !ok {
					ctx.writeNull()
					return ctx.err("returned a invalid kind of error")
				} else if err != nil {
					ctx.err(err.Error())
				}
			}
		}

		if ctx.context != nil {
			err := ctx.context.Err()
			if err != nil {
				// Context ended
				ctx.err(err.Error())
				ctx.writeNull()
				return false
			}
		}

		ctx.setGoValue(outs[method.outNr])
		criticalErr := ctx.resolveFieldDataValue(&method.outType, dept, hasSubSelection)
		return criticalErr
	case valueTypeEnum:
		enum := definedEnums[typeObj.enumTypeIndex]
		switch enum.contentKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			underlayingValue := goValue.Int()
			for _, entry := range enum.entries {
				if entry.value.Int() == underlayingValue {
					ctx.writeQouted(entry.keyBytes)
					return false
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			underlayingValue := goValue.Uint()
			for _, entry := range enum.entries {
				if entry.value.Uint() == underlayingValue {
					ctx.writeQouted(entry.keyBytes)
					return false
				}
			}
		case reflect.String:
			underlayingValue := goValue.String()
			for _, entry := range enum.entries {
				if entry.value.String() == underlayingValue {
					ctx.writeQouted(entry.keyBytes)
					return false
				}
			}
		}
		ctx.writeNull()
	case valueTypeTime:
		timeValue, ok := goValue.Interface().(time.Time)
		if ok {
			ctx.writeByte('"')
			timeToString(&ctx.result, timeValue)
			ctx.writeByte('"')
		} else {
			ctx.writeNull()
		}
	}

	return false
}

func (ctx *BytecodeCtx) valueToJson(in reflect.Value, kind reflect.Kind) {
	switch kind {
	case reflect.String:
		stringToJson(in.String(), &ctx.result)
	case reflect.Bool:
		if in.Bool() {
			ctx.write([]byte("true"))
		} else {
			ctx.write([]byte("false"))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ctx.result = strconv.AppendInt(ctx.result, in.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ctx.result = strconv.AppendUint(ctx.result, in.Uint(), 10)
	case reflect.Float32:
		floatToJson(32, in.Float(), &ctx.result)
	case reflect.Float64:
		floatToJson(64, in.Float(), &ctx.result)
	case reflect.Ptr:
		if in.IsNil() {
			ctx.writeNull()
		} else {
			element := in.Elem()
			ctx.valueToJson(element, element.Kind())
		}
	default:
		ctx.writeNull()
	}
}

// b2s converts a byte array into a string without allocating new memory
// Note that any changes to a will result in a diffrent string
func b2s(a []byte) string {
	return *(*string)(unsafe.Pointer(&a))
}
package graphql

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/mjarkk/go-graphql/bytecode"
)

type BytecodeCtx struct {
	schema *Schema
	query  bytecode.ParserCtx
	result []byte
	charNr int

	// Zero alloc values
	reflectValues          [256]reflect.Value
	currentReflectValueIdx uint8
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
	NoMeta bool
}

func (ctx *BytecodeCtx) write(b []byte) {
	ctx.result = append(ctx.result, b...)
}

func (ctx *BytecodeCtx) writeByte(b byte) {
	ctx.result = append(ctx.result, b)
}

func (ctx *BytecodeCtx) BytecodeResolve(query []byte, opts BytecodeParseOptions) ([]byte, []error) {
	*ctx = BytecodeCtx{
		schema:                 ctx.schema,
		query:                  ctx.query,
		result:                 ctx.result[:0],
		reflectValues:          ctx.reflectValues,
		currentReflectValueIdx: 0,
	}
	ctx.query.Query = append(ctx.query.Query[:0], query...)

	ctx.query.ParseQueryToBytecode()

	if !opts.NoMeta {
		ctx.write([]byte(`{"data":`))
	}

	ctx.reflectValues[0] = ctx.schema.rootQueryValue
	ctx.writeByte('{')
	ctx.resolveOperation()
	ctx.writeByte('}')

	if !opts.NoMeta {
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
	ctx.query.Errors = append(ctx.query.Errors, errors.New(msg))
	return true
}

func (ctx *BytecodeCtx) errf(msg string, args ...interface{}) bool {
	ctx.query.Errors = append(ctx.query.Errors, fmt.Errorf(msg, args...))
	return true
}

func (ctx *BytecodeCtx) resolveOperation() bool {
	ctx.charNr += 3 // read 0, [ActionOperator], [kind]

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
			// TODO not all things are queries
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

	// Read field name
	startOfName := ctx.charNr
	endOfName := ctx.charNr
	for {
		if ctx.readInst() == 0 {
			endOfName = ctx.charNr - 1
			break
		}
	}
	name := ctx.query.Res[startOfName:endOfName]
	nameStr := b2s(name)

	if ctx.readInst() != 0 {
		// TODO
		return ctx.err("field aliases not supported")
	}

	if addCommaBefore {
		ctx.writeByte(',')
	}

	ctx.writeByte('"')
	ctx.write(name)
	ctx.write([]byte{'"', ':'})

	typeObjField, ok := typeObj.objContents[nameStr]
	if !ok {
		ctx.write([]byte{'n', 'u', 'l', 'l'})
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
		ctx.write([]byte{'n', 'u', 'l', 'l'})
	case valueTypeArray:
		if (goValue.Kind() != reflect.Array && goValue.Kind() != reflect.Slice) || goValue.IsNil() {
			ctx.write([]byte("null"))
			return false
		}

		if typeObj.innerContent == nil {
			ctx.write([]byte("null"))
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
			ctx.write([]byte("null"))
			return ctx.err("must have a selection")
		}

		var ok bool
		if typeObj.valueType == valueTypeObjRef {
			typeObj, ok = ctx.schema.types[typeObj.typeName]
			if !ok {
				ctx.write([]byte("null"))
				return false
			}
		}

		ctx.writeByte('{')
		criticalErr := ctx.resolveSelectionSet(typeObj, dept)
		ctx.writeByte('}')
		return criticalErr
	case valueTypeData:
		if hasSubSelection {
			ctx.write([]byte("null"))
			return ctx.err("cannot have a selection on this field")
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
			ctx.write([]byte("null"))
		} else {
			ctx.reflectValues[ctx.currentReflectValueIdx] = goValue.Elem()
			return ctx.resolveFieldDataValue(typeObj, dept, hasSubSelection)
		}
	case valueTypeMethod:
		// TODO
		ctx.err("method value type unsupported")
		ctx.write([]byte{'n', 'u', 'l', 'l'})
	case valueTypeEnum:
		// TODO
		ctx.err("enum value type unsupported")
		ctx.write([]byte{'n', 'u', 'l', 'l'})
	case valueTypeTime:
		timeValue, ok := goValue.Interface().(time.Time)
		if ok {
			ctx.writeByte('"')
			timeToString(&ctx.result, timeValue)
			ctx.writeByte('"')
		} else {
			ctx.write([]byte("null"))
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
			ctx.write([]byte("null"))
		} else {
			element := in.Elem()
			ctx.valueToJson(element, element.Kind())
		}
	default:
		ctx.write([]byte("null"))
	}
}

// b2s converts a byte array into a string without allocating new memory
// Note that any changes to a will result in a diffrent string
func b2s(a []byte) string {
	return *(*string)(unsafe.Pointer(&a))
}

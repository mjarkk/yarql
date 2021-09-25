package graphql

import (
	"errors"
	"fmt"
	"reflect"
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
	ctx.writeByte('{')
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

	for {
		switch ctx.readInst() {
		case bytecode.ActionEnd:
			// End of operator
			return false
		case bytecode.ActionField:
			// Parse field
			// TODO not all things are queries
			criticalErr := ctx.resolveField(ctx.schema.rootQuery, 0)
			if criticalErr {
				return criticalErr
			}
		default:
			return ctx.err("unsupported operation " + string(ctx.lastInst()))
		}
	}
}

func (ctx *BytecodeCtx) resolveField(typeObj *obj, dept uint8) bool {
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
		return ctx.err("Field aliases not supported")
	}

	// Read null and end of instruction
	ctx.skipInst(2)

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

	criticalErr := ctx.resolveFieldDataValue(typeObjField, dept+1)
	ctx.currentReflectValueIdx--
	return criticalErr
}

func (ctx *BytecodeCtx) resolveFieldDataValue(typeObj *obj, dept uint8) bool {
	// goValue := ctx.getGoValue()

	ctx.write([]byte{'n', 'u', 'l', 'l'})

	return false
}

// b2s converts a byte array into a string without allocating new memory
// Note that any changes to a will result in a diffrent string
func b2s(a []byte) string {
	return *(*string)(unsafe.Pointer(&a))
}

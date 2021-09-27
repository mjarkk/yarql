package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/mjarkk/go-graphql/bytecode"
)

type BytecodeCtx struct {
	// private
	schema              *Schema
	query               bytecode.ParserCtx
	charNr              int
	context             context.Context
	path                []byte
	getFormFile         func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
	operatorArgumentsAt int

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
	Values         map[string]interface{}                          // Passed directly to the request context
	GetFormFile    func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading

	// TODO support:
	// Variables      string // Expects valid JSON or empty string
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

// returns the path json encoded
func (ctx *BytecodeCtx) GetPath() json.RawMessage {
	if len(ctx.path) == 0 {
		return []byte("[]")
	}
	return append(append([]byte{'['}, ctx.path[1:]...), ']')
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
		schema:      ctx.schema,
		query:       ctx.query,
		charNr:      0,
		context:     opts.Context,
		path:        ctx.path[:0],
		getFormFile: opts.GetFormFile,

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
		// Write add errors to output
		ctx.write([]byte(`,"errors":[`))
		for i, err := range ctx.query.Errors {
			if i > 0 {
				ctx.writeByte(',')
			}
			ctx.write([]byte(`{"message":`))
			stringToJson(err.Error(), &ctx.result)

			errWPath, isErrWPath := err.(ErrorWPath)
			if isErrWPath && len(errWPath.path) > 0 {
				ctx.write([]byte(`,"path":[`))
				ctx.write(errWPath.path)
				ctx.writeByte(']')
			}
			errWLocation, isErrWLocation := err.(ErrorWLocation)
			if isErrWLocation {
				ctx.write([]byte(`,"locations":[{"line":`))
				ctx.result = strconv.AppendUint(ctx.result, uint64(errWLocation.line), 10)
				ctx.write([]byte(`,"column":`))
				ctx.result = strconv.AppendUint(ctx.result, uint64(errWLocation.column), 10)
				ctx.write([]byte(`}]`))
			}
			ctx.writeByte('}')
		}

		// TODO support content for the extensions map
		ctx.write([]byte(`],"extensions":{}`))

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

func (ctx *BytecodeCtx) err(msg string) bool {
	err := errors.New(msg)
	if len(ctx.path) == 0 {
		ctx.query.Errors = append(ctx.query.Errors, err)
	} else {
		copiedPath := make([]byte, len(ctx.path)-1)
		copy(copiedPath, ctx.path[1:])

		ctx.query.Errors = append(ctx.query.Errors, ErrorWPath{
			err:  err,
			path: copiedPath,
		})
	}
	return true
}

func (ctx *BytecodeCtx) errf(msg string, args ...interface{}) bool {
	return ctx.err(fmt.Sprintf(msg, args...))
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

	hasArguments := ctx.readInst() == 't'
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

	if hasArguments {
		argsEndAt := ctx.query.Res[ctx.charNr : ctx.charNr+4]

		argsEndAtNr := uint32(argsEndAt[0]) |
			(uint32(argsEndAt[1]) << 8) |
			(uint32(argsEndAt[2]) << 16) |
			(uint32(argsEndAt[3]) << 24)

		// Skip over arguments end location and null byte
		ctx.skipInst(5)
		ctx.operatorArgumentsAt = ctx.charNr

		ctx.charNr = int(argsEndAtNr) + 1
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
	var endOfAlias int
	for {
		if ctx.readInst() == 0 {
			endOfAlias = ctx.charNr - 1
			break
		}
	}
	alias := ctx.query.Res[startOfAlias:endOfAlias]

	prefPathLen := len(ctx.path)
	ctx.path = append(ctx.path, []byte(`,"`)...)
	ctx.path = append(ctx.path, alias...)
	ctx.path = append(ctx.path, '"')
	// Note that from here on we should restore the path on errors

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

	criticalErr := false
	fieldHasSelection := ctx.seekInst() != 'e'

	if nameStr == "__typename" {
		if fieldHasSelection {
			criticalErr = ctx.err("cannot have a selection set on this field")
		} else {
			ctx.writeQouted(typeObj.typeNameBytes)
		}
	} else {
		typeObjField, ok := typeObj.objContents[nameStr]
		if !ok {
			ctx.writeNull()
			criticalErr = ctx.errf("%s does not exists on %s", nameStr, typeObj.typeName)
		} else {
			goValue := ctx.getGoValue()
			if typeObjField.customObjValue != nil {
				ctx.setNextGoValue(*typeObjField.customObjValue)
			} else if typeObjField.valueType == valueTypeMethod && typeObjField.method.isTypeMethod {
				ctx.setNextGoValue(goValue.Method(typeObjField.structFieldIdx))
			} else {
				ctx.setNextGoValue(goValue.Field(typeObjField.structFieldIdx))
			}

			criticalErr = ctx.resolveFieldDataValue(typeObjField, dept, fieldHasSelection)
			ctx.currentReflectValueIdx--
		}
	}

	// Restore the path
	ctx.path = ctx.path[:prefPathLen]

	if criticalErr {
		return criticalErr
	}

	inst := ctx.readInst()
	if inst == bytecode.ActionEnd {
		if ctx.readInst() != 0 {
			return ctx.errf("internal parsing error #2, %v", ctx.lastInst())
		}
	} else if inst != 0 {
		return ctx.errf("internal parsing error #1, %v = %s", ctx.lastInst(), string(ctx.lastInst()))
	}
	return false
}

func (ctx *BytecodeCtx) resolveFieldDataValue(typeObj *obj, dept uint8, hasSubSelection bool) bool {
	goValue := ctx.getGoValue()
	if ctx.seekInst() == bytecode.ActionValue && typeObj.valueType != valueTypeMethod {
		// Check there is no method behind a pointer
		resolvedTypeObj := typeObj
		for resolvedTypeObj.valueType == valueTypePtr {
			resolvedTypeObj = resolvedTypeObj.innerContent
		}

		if resolvedTypeObj.valueType != valueTypeMethod {
			// arguments are not allowed on any other value than methods
			ctx.writeNull()
			return ctx.err("field arguments not allowed")
		}
	}

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

			prefPathLen := len(ctx.path)
			ctx.path = append(ctx.path, ',')
			ctx.path = strconv.AppendInt(ctx.path, int64(i), 10)

			ctx.setGoValue(goValue.Index(i))

			ctx.resolveFieldDataValue(typeObj, dept, hasSubSelection)
			if i < goValueLen-1 {
				ctx.writeByte(',')
			}

			ctx.path = ctx.path[:prefPathLen]
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
			return ctx.resolveFieldDataValue(typeObj.innerContent, dept, hasSubSelection)
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
				// TODO this is a dirty hack to get the context working again
				// With this hack extensions and the errors are not added to our context when the user calls those methods
				ctx.schema.ctx.errors = ctx.query.Errors
				ctx.schema.ctx.path = ctx.path
				ctx.schema.ctx.extensions = map[string]interface{}{}
				ctx.schema.ctx.context = ctx.context

				ctx.funcInputs = append(ctx.funcInputs, ctx.schema.ctxReflection)
			} else {
				ctx.funcInputs = append(ctx.funcInputs, reflect.New(*in.type_).Elem())
			}
		}

		if ctx.seekInst() == 'v' {
			criticalErr := ctx.walkInputObject(
				func(key []byte) bool {
					keyStr := b2s(key)
					inField, ok := method.inFields[keyStr]
					if !ok {
						return ctx.err("undefined input: " + keyStr)
					}
					goField := ctx.funcInputs[inField.inputIdx].Field(inField.input.goFieldIdx)
					return ctx.bindInputToGoValue(&goField, &inField.input)
				},
			)
			if criticalErr {
				return criticalErr
			}
			hasSubSelection = ctx.seekInst() != 'e'
		}

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

func (ctx *BytecodeCtx) bindInputToGoValue(goValue *reflect.Value, valueStructure *input) bool {
	// TODO convert to go value kind to graphql value kind in errors

	if goValue.Kind() == reflect.Ptr {
		if ctx.query.Res[ctx.charNr+1] == bytecode.ValueNull {
			// keep goValue at it's default
			ctx.skipInst(2)
			return false
		}

		goValueElem := goValue.Type().Elem()
		newVal := reflect.New(goValueElem)
		newValElem := newVal.Elem()

		criticalErr := ctx.bindInputToGoValue(&newValElem, valueStructure.elem)
		if criticalErr {
			return criticalErr
		}

		goValue.Set(newVal)
		return false
	}

	getValue := func() (start int, end int) {
		start = ctx.charNr
		for {
			if ctx.readInst() == 0 {
				return start, ctx.charNr - 1
			}
		}
	}

	if ctx.query.Res[ctx.charNr+1] == bytecode.ValueVariable {
		// TODO
		return ctx.err("variable input value kind unsupported")

		// varNameStart, varNameEnd := getValue()
		// varName := b2s(ctx.query.Res[varNameStart:varNameEnd])
	}

	ctx.skipInst(1) // read ActionValue
	switch ctx.readInst() {
	case bytecode.ValueInt:
		startInt, endInt := getValue()
		intValue := b2s(ctx.query.Res[startInt:endInt])

		switch goValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value, err := strconv.ParseInt(intValue, 10, 64)
			if err != nil {
				return ctx.err(err.Error())
			}

			switch goValue.Kind() {
			case reflect.Int8:
				if int64(int8(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 8bit intager")
				}
			case reflect.Int16:
				if int64(int16(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 16bit intager")
				}
			case reflect.Int32:
				if int64(int32(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 32bit intager")
				}
			}

			goValue.SetInt(value)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value, err := strconv.ParseUint(intValue, 10, 64)
			if err != nil {
				return ctx.err(err.Error())
			}

			switch goValue.Kind() {
			case reflect.Uint8:
				if uint64(uint8(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 8bit unsigned intager")
				}
			case reflect.Uint16:
				if uint64(uint16(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 16bit unsigned intager")
				}
			case reflect.Uint32:
				if uint64(uint32(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 32bit unsigned intager")
				}
			}

			goValue.SetUint(value)
		case reflect.Float32, reflect.Float64:
			value, err := strconv.ParseInt(intValue, 10, 64)
			if err != nil {
				return ctx.err(err.Error())
			}

			goValue.SetFloat(float64(value))
		case reflect.Bool:
			value, err := strconv.ParseInt(intValue, 10, 64)
			if err != nil {
				return ctx.err(err.Error())
			}

			goValue.SetBool(value > 0)
		default:
			return ctx.err("cannot assign int to " + goValue.String())
		}

	case bytecode.ValueFloat:
		switch goValue.Kind() {
		case reflect.Float32, reflect.Float64:
			startFloat, endFloat := getValue()
			floatValue, err := strconv.ParseFloat(b2s(ctx.query.Res[startFloat:endFloat]), 64)
			if err != nil {
				return ctx.err(err.Error())
			}

			goValue.SetFloat(floatValue)
		default:
			return ctx.err("cannot assign float to " + goValue.String())
		}
	case bytecode.ValueString:
		startString, endString := getValue()
		stringValue := b2s(ctx.query.Res[startString:endString])

		if valueStructure.isID {
			switch goValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				intValue, err := strconv.Atoi(stringValue)
				if err != nil {
					return ctx.err("id argument must match a number type")
				}
				goValue.SetInt(int64(intValue))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				intValue, err := strconv.Atoi(stringValue)
				if err != nil {
					return ctx.err("id argument must match a number type")
				}
				if intValue < 0 {
					return ctx.err("id argument must match a number above 0")
				}
				goValue.SetUint(uint64(intValue))
			default:
				return ctx.err("cannot assign string to ID field")
			}
		} else if valueStructure.isFile {
			if ctx.getFormFile == nil {
				return ctx.err("form files are not supported")
			}
			file, err := ctx.getFormFile(stringValue)
			if err != nil {
				return ctx.err(err.Error())
			}
			goValue.Set(reflect.ValueOf(file))
		} else if valueStructure.isTime {
			parsedTime, err := parseTime(stringValue)
			if err != nil {
				return ctx.err(err.Error())
			}
			goValue.Set(reflect.ValueOf(parsedTime))
		} else if goValue.Kind() == reflect.String {
			goValue.SetString(stringValue)
		} else {
			return ctx.err("cannot assign string to " + goValue.String())
		}
	case bytecode.ValueBoolean:
		if goValue.Kind() != reflect.Bool {
			return ctx.err("cannot assign boolean to " + goValue.String())
		}
		goValue.SetBool(ctx.readInst() == '1')
		ctx.skipInst(1)
	case bytecode.ValueNull:
		// keep goValue at it's default
		ctx.skipInst(1)
	case bytecode.ValueEnum:
		if !valueStructure.isEnum {
			return ctx.err("cannot assign enum to non enum value")
		}

		nameStart, nameEnd := getValue()
		name := b2s(ctx.query.Res[nameStart:nameEnd])

		enum := definedEnums[valueStructure.enumTypeIndex]
		for _, entry := range enum.entries {
			if entry.key == name {
				switch enum.contentKind {
				case reflect.String:
					goValue.SetString(entry.value.String())
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					goValue.SetInt(entry.value.Int())
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					goValue.SetUint(entry.value.Uint())
				default:
					return ctx.err("internal error, type missmatch on enum")
				}
				return false
			}
		}

		return ctx.errf("unknown enum value %s for enum %s", name, enum.typeName)
	case bytecode.ValueList:
		goValueKind := goValue.Kind()
		if goValueKind == reflect.Array {
			// TODO support this
			return ctx.err("fixed length arrays not supported")
		}
		if goValueKind != reflect.Slice {
			return ctx.err("cannot assign list to " + goValue.String())
		}

		arr := reflect.MakeSlice(goValue.Type(), 0, 0)
		arrItemType := arr.Type().Elem()

		ctx.skipInst(1) // read NULL
		for ctx.seekInst() != 'e' {
			arrayEntry := reflect.New(arrItemType).Elem()
			criticalErr := ctx.bindInputToGoValue(&arrayEntry, valueStructure.elem)
			if criticalErr {
				return criticalErr
			}
			arr = reflect.Append(arr, arrayEntry)
		}

		goValue.Set(arr)
	case bytecode.ValueObject:
		if goValue.Kind() != reflect.Struct {
			return ctx.err("cannot assign object to " + goValue.String())
		}

		if valueStructure.isStructPointers {
			valueStructure = ctx.schema.inTypes[valueStructure.structName]
		}

		// walkInputObject expects to start at ActionValue while we just read over it
		ctx.skipInst(-2)

		criticalErr := ctx.walkInputObject(func(key []byte) bool {
			structFieldValueStructure, ok := valueStructure.structContent[b2s(key)]
			if !ok {
				return ctx.err("undefined property " + b2s(key))
			}

			field := goValue.Field(structFieldValueStructure.goFieldIdx)
			return ctx.bindInputToGoValue(&field, &structFieldValueStructure)
		})
		if criticalErr {
			return criticalErr
		}
	}
	return false
}

// walkInputObject walks over an input object and triggers onValueOfKey after reading a key and reached it value
// onValueOfKey is expected to parse the value before returning
func (ctx *BytecodeCtx) walkInputObject(onValueOfKey func(key []byte) bool) bool {
	// Read ActionValue and ValueObject and NULL
	ctx.skipInst(3)

	for {
		// Check if the current or next value is the end
		c := ctx.readInst()
		if c == 'e' || c == 0 && ctx.readInst() == 'e' {
			// end of object
			ctx.skipInst(1) // skip next NULL byte
			return false
		}
		keyStart := ctx.charNr
		var keyEnd int
		for {
			if ctx.readInst() == 0 {
				keyEnd = ctx.charNr - 1
				break
			}
		}
		key := ctx.query.Res[keyStart:keyEnd]
		criticalErr := onValueOfKey(key)
		if criticalErr {
			return criticalErr
		}
	}
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

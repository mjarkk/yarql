package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"mime/multipart"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/mjarkk/go-graphql/bytecode"
	"github.com/mjarkk/go-graphql/helpers"
	"github.com/valyala/fastjson"
)

// Ctx contains all runtime information of a query
type Ctx struct {
	// private
	schema                   *Schema
	query                    bytecode.ParserCtx
	charNr                   int
	context                  context.Context
	path                     []byte
	getFormFile              func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
	operatorArgumentsStartAt int
	tracingEnabled           bool
	tracing                  *tracer
	prefRecordingStartTime   time.Time

	rawVariables        string
	variablesParsed     bool             // the rawVariables are parsed into variables
	variablesJSONParser *fastjson.Parser // Used to parse the variables
	variables           *fastjson.Value  // Parsed variables, only use this if variablesParsed == true

	// Zero alloc values
	reflectValues          [256]reflect.Value
	currentReflectValueIdx uint8
	funcInputs             []reflect.Value
	ctxReflection          reflect.Value // ptr to the value

	// public / kinda public fields
	values *map[string]interface{} // API User values, user can put all their shitty things in here like poems or tax papers
}

func newCtx(s *Schema) *Ctx {
	ctx := &Ctx{
		schema: s,
		query: bytecode.ParserCtx{
			Res:               make([]byte, 2048),
			FragmentLocations: make([]int, 8),
			Query:             make([]byte, 2048),
			Errors:            []error{},
			Hasher:            fnv.New32(),
		},
		charNr:                 0,
		reflectValues:          [256]reflect.Value{},
		currentReflectValueIdx: 0,
		variablesJSONParser:    &fastjson.Parser{},
		tracing:                newTracer(),
	}
	ctx.ctxReflection = reflect.ValueOf(ctx)
	return ctx
}

func (ctx *Ctx) getGoValue() reflect.Value {
	return ctx.reflectValues[ctx.currentReflectValueIdx]
}

func (ctx *Ctx) setNextGoValue(value reflect.Value) {
	ctx.currentReflectValueIdx++
	ctx.setGoValue(value)
}

func (ctx *Ctx) setGoValue(value reflect.Value) {
	ctx.reflectValues[ctx.currentReflectValueIdx] = value
}

// GetValue returns a user defined value
func (ctx *Ctx) GetValue(key string) (value interface{}) {
	if ctx.values == nil {
		return nil
	}
	return (*ctx.values)[key]
}

// GetValueOk returns a user defined value with a boolean indicating if the value was found
func (ctx *Ctx) GetValueOk(key string) (value interface{}, found bool) {
	if ctx.values == nil {
		return nil, false
	}
	val, ok := (*ctx.values)[key]
	return val, ok
}

// SetValue sets a user defined value
func (ctx *Ctx) SetValue(key string, value interface{}) {
	if ctx.values == nil {
		ctx.values = &map[string]interface{}{
			key: value,
		}
	} else {
		(*ctx.values)[key] = value
	}
}

// GetPath returns the graphql path to the current field json encoded
func (ctx *Ctx) GetPath() json.RawMessage {
	if len(ctx.path) == 0 {
		return []byte("[]")
	}
	return append(append([]byte{'['}, ctx.path[1:]...), ']')
}

func (ctx *Ctx) write(b []byte) {
	ctx.schema.Result = append(ctx.schema.Result, b...)
}

func (ctx *Ctx) writeByte(b byte) {
	ctx.schema.Result = append(ctx.schema.Result, b)
}

func (ctx *Ctx) writeQouted(b []byte) {
	ctx.writeByte('"')
	ctx.write(b)
	ctx.writeByte('"')
}

var nullBytes = []byte("null")

func (ctx *Ctx) writeNull() {
	ctx.write(nullBytes)
}

type ResolveOptions struct {
	NoMeta         bool            // Returns only the data
	Context        context.Context // Request context
	OperatorTarget string
	Values         *map[string]interface{}                         // Passed directly to the request context
	GetFormFile    func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
	Variables      string                                          // Expects valid JSON or empty string
	Tracing        bool                                            // https://github.com/apollographql/apollo-tracing
}

func (s *Schema) Resolve(query []byte, opts ResolveOptions) []error {
	if !s.parsed {
		fmt.Println("CALL (*graphql.Schema).Parse() before resolve")
		return []error{errors.New("invalid setup")}
	}

	s.Result = s.Result[:0]

	ctx := s.ctx
	*ctx = Ctx{
		schema:                 ctx.schema,
		query:                  ctx.query,
		charNr:                 0,
		context:                opts.Context,
		path:                   ctx.path[:0],
		getFormFile:            opts.GetFormFile,
		rawVariables:           opts.Variables,
		variablesParsed:        false,
		variablesJSONParser:    ctx.variablesJSONParser,
		variables:              ctx.variables,
		tracingEnabled:         opts.Tracing,
		tracing:                ctx.tracing,
		prefRecordingStartTime: ctx.prefRecordingStartTime,
		ctxReflection:          ctx.ctxReflection,

		reflectValues:          ctx.reflectValues,
		currentReflectValueIdx: 0,
		funcInputs:             ctx.funcInputs,

		values: opts.Values,
	}
	if opts.Tracing {
		ctx.tracing.reset()
	}
	ctx.startTrace()

	ctx.query.Query = append(ctx.query.Query[:0], query...)

	if len(opts.OperatorTarget) > 0 {
		ctx.query.ParseQueryToBytecode(&opts.OperatorTarget)
	} else {
		ctx.query.ParseQueryToBytecode(nil)
	}

	if ctx.tracingEnabled {
		// finish parsing trace
		ctx.finishTrace(func(offset, duration int64) {
			ctx.tracing.Parsing.StartOffset = offset
			ctx.tracing.Parsing.Duration = duration
		})

		// Set the validation prop
		ctx.tracing.Validation.StartOffset = time.Now().Sub(ctx.prefRecordingStartTime).Nanoseconds()
	}

	if !opts.NoMeta {
		ctx.write([]byte(`{"data":`))
	}

	if len(ctx.query.Errors) == 0 {
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
	} else {
		ctx.write([]byte("{}"))
	}

	if !opts.NoMeta {
		// TODO support custom extensions

		// Add errors to output
		errsLen := len(ctx.query.Errors)
		if errsLen == 0 && !ctx.tracingEnabled {
			ctx.write([]byte(`,"errors":[],"extensions":{}}`))
		} else {
			if errsLen == 0 {
				ctx.write([]byte(`,"errors":[]`))
			} else {
				ctx.write([]byte(`,"errors":[`))
				for i, err := range ctx.query.Errors {
					if i > 0 {
						ctx.writeByte(',')
					}
					ctx.write([]byte(`{"message":`))
					stringToJson(err.Error(), &ctx.schema.Result)

					errWPath, isErrWPath := err.(ErrorWPath)
					if isErrWPath && len(errWPath.path) > 0 {
						ctx.write([]byte(`,"path":[`))
						ctx.write(errWPath.path)
						ctx.writeByte(']')
					}
					errWLocation, isErrWLocation := err.(bytecode.ErrorWLocation)
					if isErrWLocation {
						ctx.write([]byte(`,"locations":[{"line":`))
						ctx.schema.Result = strconv.AppendUint(ctx.schema.Result, uint64(errWLocation.Line), 10)
						ctx.write([]byte(`,"column":`))
						ctx.schema.Result = strconv.AppendUint(ctx.schema.Result, uint64(errWLocation.Column), 10)
						ctx.write([]byte{'}', ']'})
					}
					ctx.writeByte('}')
				}
				ctx.writeByte(']')
			}

			if ctx.tracingEnabled {
				ctx.write([]byte(`,"extensions":{"tracing":`))
				ctx.tracing.finish()
				tracingJSON, err := json.Marshal(ctx.tracing)
				if err == nil {
					ctx.write(tracingJSON)
				} else {
					ctx.writeNull()
				}
				ctx.write([]byte{'}', '}'})
			} else {
				ctx.write([]byte(`,"extensions":{}}`))
			}
		}
	}

	return ctx.query.Errors
}

// readInst reads the current instruction and increments the charNr
func (ctx *Ctx) readInst() byte {
	c := ctx.query.Res[ctx.charNr]
	ctx.charNr++
	return c
}

func (ctx *Ctx) seekInst() byte {
	return ctx.query.Res[ctx.charNr]
}

func (ctx *Ctx) skipInst(num int) {
	ctx.charNr += num
}

func (ctx *Ctx) lastInst() byte {
	return ctx.query.Res[ctx.charNr-1]
}

// ErrorWPath is an error mesage with a graphql path to the field that created the error
type ErrorWPath struct {
	err  error
	path []byte // a json representation of the path without the [] around it
}

func (e ErrorWPath) Error() string {
	return e.err.Error()
}

func (ctx *Ctx) err(msg string) bool {
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

func (ctx *Ctx) errf(msg string, args ...interface{}) bool {
	return ctx.err(fmt.Sprintf(msg, args...))
}

func (ctx *Ctx) readUint32(startAt int) uint32 {
	data := ctx.query.Res[startAt : startAt+4]
	return uint32(data[0]) |
		(uint32(data[1]) << 8) |
		(uint32(data[2]) << 16) |
		(uint32(data[3]) << 24)
}

func (ctx *Ctx) resolveOperation() bool {
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
		argumentsLen := ctx.readUint32(ctx.charNr)

		// Skip over arguments end location and null byte
		ctx.operatorArgumentsStartAt = ctx.charNr + 5
		ctx.skipInst(int(argumentsLen) + 5)
	}

	firstField := true
	if kind == bytecode.OperatorMutation {
		return ctx.resolveSelectionSet(ctx.schema.rootMethod, 0, &firstField)
	}
	return ctx.resolveSelectionSet(ctx.schema.rootQuery, 0, &firstField)
}

func (ctx *Ctx) resolveSelectionSet(typeObj *obj, dept uint8, firstField *bool) bool {
	for {
		switch ctx.readInst() {
		case bytecode.ActionEnd:
			// End of operator
			return false
		case bytecode.ActionField:
			// Parse field
			skipped, criticalErr := ctx.resolveField(typeObj, dept, !*firstField)
			if criticalErr {
				return criticalErr
			}
			if !skipped {
				*firstField = false
			}
		case bytecode.ActionSpread:
			criticalErr := ctx.resolveSpread(typeObj, dept, firstField)
			if criticalErr {
				return criticalErr
			}
		default:
			return ctx.err("unsupported operation " + string(ctx.lastInst()))
		}
	}
}

func (ctx *Ctx) resolveSpread(typeObj *obj, dept uint8, firstField *bool) bool {
	isInline := ctx.readInst() == 't'
	directivesCount := ctx.readInst()

	lenOfDirective := ctx.readUint32(ctx.charNr)
	ctx.skipInst(4)

	// Read name
	nameStart := ctx.charNr
	var endName int
	for {
		if ctx.readInst() == 0 {
			endName = ctx.charNr - 1
			break
		}
	}

	if directivesCount != 0 {
		location := DirectiveLocationFragment
		if isInline {
			location = DirectiveLocationFragmentInline
		}

		for i := uint8(0); i < directivesCount; i++ {
			modifer, criticalErr := ctx.resolveDirective(location)
			if criticalErr || modifer.Skip {
				ctx.charNr = nameStart + int(lenOfDirective) + 1
				return criticalErr
			}
		}
	}

	if isInline {
		criticalErr := ctx.resolveSelectionSet(typeObj, dept, firstField)
		ctx.charNr++
		return criticalErr
	}

	nameLen := endName - nameStart
	name := ctx.query.Res[nameStart:endName]
	ctxQueryResLen := len(ctx.query.Res)

	for _, location := range ctx.query.FragmentLocations {
		fragmentNameStart := location + 1
		fragmentNameEnd := fragmentNameStart + nameLen
		if fragmentNameEnd >= ctxQueryResLen {
			continue
		}
		if bytes.Equal(ctx.query.Res[fragmentNameStart:fragmentNameEnd], name) {
			originalCharNr := ctx.charNr
			ctx.charNr = fragmentNameEnd + 2

			// Read the type
			for {
				if ctx.readInst() == 0 {
					break
				}
			}

			criticalErr := ctx.resolveSelectionSet(typeObj, dept, firstField)
			ctx.charNr = originalCharNr
			return criticalErr
		}
	}

	return ctx.err("fragment " + b2s(name) + " not defined")
}

func (ctx *Ctx) resolveField(typeObj *obj, dept uint8, addCommaBefore bool) (skipped bool, criticalErr bool) {
	ctx.startTrace()

	directivesCount := ctx.readInst()

	fieldLen := ctx.readUint32(ctx.charNr)
	ctx.skipInst(4)
	nameKey := ctx.readUint32(ctx.charNr)
	ctx.skipInst(4)
	endOfField := ctx.charNr + int(fieldLen)

	// Read field name/alias
	aliasLen := int(ctx.readInst())
	startOfAlias := ctx.charNr
	endOfAlias := startOfAlias + aliasLen
	alias := ctx.query.Res[startOfAlias:endOfAlias]
	ctx.skipInst(aliasLen)

	prefPathLen := len(ctx.path)
	ctx.path = append(ctx.path, []byte(`,"`)...)
	ctx.path = append(ctx.path, alias...)
	ctx.path = append(ctx.path, '"')
	// Note that from here on we should restore the path on errors

	startOfName := startOfAlias
	endOfName := endOfAlias

	// If alias is used read the name
	lenOfName := ctx.readInst()
	if lenOfName != 0 {
		startOfName = ctx.charNr
		endOfName = startOfName + int(lenOfName)
		ctx.skipInst(int(lenOfName))
	}
	ctx.skipInst(1)

	if directivesCount != 0 {
		for i := uint8(0); i < directivesCount; i++ {
			modifier, criticalErr := ctx.resolveDirective(DirectiveLocationField)

			if criticalErr || modifier.Skip {
				// Restore the path
				ctx.path = ctx.path[:prefPathLen]
				ctx.charNr = endOfField + 1

				return true, criticalErr
			}

			// TODO
			// if modifier.ModifyOnWriteContent != nil {
			// 	contentModifiers = append(contentModifiers, modifier.ModifyOnWriteContent)
			// }
		}
	}

	if addCommaBefore {
		ctx.writeByte(',')
	}

	ctx.writeQouted(alias)
	ctx.writeByte(':')

	fieldHasSelection := ctx.seekInst() != 'e'

	typeObjField, ok := typeObj.objContents[nameKey]
	if !ok {
		name := b2s(ctx.query.Res[startOfName:endOfName])
		if name == "__typename" {
			if fieldHasSelection {
				criticalErr = ctx.err("cannot have a selection set on this field")
			} else {
				ctx.writeQouted(typeObj.typeNameBytes)
			}
		} else {
			ctx.writeNull()
			criticalErr = ctx.errf("%s does not exists on %s", name, typeObj.typeName)
		}
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

	if ctx.tracingEnabled {
		name := b2s(ctx.query.Res[startOfName:endOfName])

		ctx.finishTrace(func(offset, duration int64) {
			returnType := bytes.NewBuffer(nil)
			ctx.schema.objToQlTypeName(typeObjField, returnType)

			ctx.tracing.Execution.Resolvers = append(ctx.tracing.Execution.Resolvers, tracerResolver{
				Path:        ctx.GetPath(),
				ParentType:  typeObj.typeName,
				FieldName:   name,
				ReturnType:  returnType.String(),
				StartOffset: offset,
				Duration:    duration,
			})
		})
	}

	// Restore the path
	ctx.path = ctx.path[:prefPathLen]

	ctx.charNr = endOfField + 1

	return false, criticalErr
}

func (ctx *Ctx) callQlMethod(method *objMethod, goValue *reflect.Value, parseArguments bool) ([]reflect.Value, bool) {
	ctx.funcInputs = ctx.funcInputs[:0]
	for _, in := range method.ins {
		if in.isCtx {
			ctx.funcInputs = append(ctx.funcInputs, ctx.ctxReflection)
		} else {
			ctx.funcInputs = append(ctx.funcInputs, reflect.New(*in.type_).Elem())
		}
	}

	if parseArguments {
		criticalErr := ctx.walkInputObject(
			func(key []byte) bool {
				keyStr := b2s(key)
				inField, ok := method.inFields[keyStr]
				if !ok {
					return ctx.err("undefined input: " + keyStr)
				}
				goField := ctx.funcInputs[inField.inputIdx].Field(inField.input.goFieldIdx)
				return ctx.bindInputToGoValue(&goField, &inField.input, true)
			},
		)
		if criticalErr {
			return nil, criticalErr
		}
	}

	outs := goValue.Call(ctx.funcInputs)
	return outs, false
}

func (ctx *Ctx) resolveDirective(location DirectiveLocation) (modifer DirectiveModifier, criticalErr bool) {
	ctx.skipInst(1) // read 'd'
	hasArguments := ctx.readInst() == 't'

	// Read name
	nameStart := ctx.charNr
	var nameEnd int
	for {
		if ctx.readInst() == 0 {
			nameEnd = ctx.charNr - 1
			break
		}
	}
	directiveName := b2s(ctx.query.Res[nameStart:nameEnd])
	directives, ok := ctx.schema.definedDirectives[location]
	if !ok {
		return modifer, ctx.err("unknown directive " + directiveName)
	}

	var foundDirective *Directive
	for _, directive := range directives {
		if directive.Name == directiveName {
			foundDirective = directive
			break
		}
	}

	if foundDirective == nil {
		return modifer, ctx.err("unknown directive " + directiveName)
	}
	method := foundDirective.parsedMethod

	outs, criticalErr := ctx.callQlMethod(method, &foundDirective.methodReflection, hasArguments)
	if criticalErr {
		return modifer, criticalErr
	}

	modifer = outs[0].Interface().(DirectiveModifier)
	return modifer, false
}

func (ctx *Ctx) resolveFieldDataValue(typeObj *obj, dept uint8, hasSubSelection bool) bool {
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
		// Using unsafe.Pointer(goValue.Pointer()) instead of goValue.isNil as it is faster
		if goValue.Kind() == reflect.Slice && unsafe.Pointer(goValue.Pointer()) == nil {
			ctx.writeNull()
			return false
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
			if i != goValueLen-1 {
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

		dept++
		if dept == ctx.schema.MaxDepth {
			ctx.writeNull()
			return ctx.err("reached max dept")
		}

		ctx.writeByte('{')
		isFirstField := true
		criticalErr := ctx.resolveSelectionSet(typeObj, dept, &isFirstField)
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
		if goValue.IsNil() {
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

		outs, criticalErr := ctx.callQlMethod(method, &goValue, ctx.seekInst() == 'v')
		if criticalErr {
			return criticalErr
		}

		hasSubSelection = ctx.seekInst() != 'e'
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
		criticalErr = ctx.resolveFieldDataValue(&method.outType, dept, hasSubSelection)
		return criticalErr
	case valueTypeEnum:
		enum := ctx.schema.definedEnums[typeObj.enumTypeIndex]
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
			helpers.TimeToIso8601String(&ctx.schema.Result, timeValue)
			ctx.writeByte('"')
		} else {
			ctx.writeNull()
		}
	case valueTypeInterface, valueTypeInterfaceRef:
		if !hasSubSelection {
			ctx.writeNull()
			return ctx.err("must have a selection")
		}

		var ok bool
		if typeObj.valueType == valueTypeInterfaceRef {
			typeObj, ok = ctx.schema.interfaces[typeObj.typeName]
			if !ok {
				ctx.writeNull()
				return false
			}
		}

		if goValue.IsNil() {
			ctx.writeNull()
			return false
		}

		if goValue.Kind() == reflect.Interface {
			goValue = goValue.Elem()
		}

		goValueType := goValue.Type()
		goValueName := goValueType.Name()
		goValuePkgPath := goValueType.PkgPath()

		// TODO improve performance of the below
		for _, implementation := range typeObj.implementations {
			if implementation.goTypeName == goValueName && implementation.goPkgPath == goValuePkgPath {
				return ctx.resolveFieldDataValue(implementation, dept, hasSubSelection)
			}
		}
		ctx.writeNull()
	}

	return false
}

func (ctx *Ctx) findOperatorArgument(nameToFind string) (foundArgument bool) {
	ctx.charNr = ctx.operatorArgumentsStartAt
	ctx.skipInst(2)
	for {
		startOfArg := ctx.charNr
		if ctx.readInst() == 'e' {
			return false
		}
		argLen := ctx.readUint32(ctx.charNr)
		ctx.charNr += 4

		nameStart := ctx.charNr
		var nameEnd int
		for {
			if ctx.readInst() == 0 {
				nameEnd = ctx.charNr - 1
				break
			}
		}

		name := b2s(ctx.query.Res[nameStart:nameEnd])
		if name == nameToFind {
			return true
		}

		ctx.charNr = startOfArg + int(argLen) + 1
	}
}

func (ctx *Ctx) bindOperatorArgumentTo(goValue *reflect.Value, valueStructure *input, argumentName string) bool {
	// TODO Check for the required flag (L & N)
	// These flags are to identify if the argument is required or not

	// TODO the error messages in this function are garbage

	resolvedValueStructure := valueStructure
	c := ctx.readInst()
	for {
		if c != 'L' && c != 'l' {
			break
		}
		if resolvedValueStructure.kind != reflect.Slice {
			return ctx.err("variable $" + argumentName + " cannot be bind to " + resolvedValueStructure.kind.String())
		}
		resolvedValueStructure = resolvedValueStructure.elem
		c = ctx.readInst()
	}
	if c == 'n' || c == 'N' {
		// N = required type
		// n = not required type

		typeNameStart := ctx.charNr
		var typeNameEnd int
		for {
			if ctx.readInst() == 0 {
				typeNameEnd = ctx.charNr - 1
				break
			}
		}

		typeName := b2s(ctx.query.Res[typeNameStart:typeNameEnd])
		if resolvedValueStructure.isEnum {
			enum := ctx.schema.definedEnums[resolvedValueStructure.enumTypeIndex]
			if typeName != enum.typeName && typeName != "String" {
				return ctx.err("expected variable type " + enum.typeName + " but got " + typeName)
			}
		} else if resolvedValueStructure.isID {
			if typeName != "ID" && typeName != "String" {
				return ctx.err("expected variable type ID but got " + typeName)
			}
		} else if resolvedValueStructure.isFile {
			if typeName != "File" && typeName != "String" {
				return ctx.err("expected variable type File but got " + typeName)
			}
		} else if resolvedValueStructure.isTime {
			if typeName != "Time" && typeName != "String" {
				return ctx.err("expected variable type Time but got " + typeName)
			}
		} else {
			switch resolvedValueStructure.kind {
			case reflect.Bool:
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if typeName != "Int" {
					return ctx.err("expected variable type Int but got " + typeName)
				}
			case reflect.Float32, reflect.Float64:
				if typeName != "Float" {
					return ctx.err("expected variable type Float but got " + typeName)
				}
			case reflect.Array, reflect.Slice:
				if typeName != "List" {
					return ctx.err("expected variable type List but got " + typeName)
				}
			case reflect.String:
				if typeName != "String" {
					return ctx.err("expected variable type String but got " + typeName)
				}
			case reflect.Struct:
				if typeName != resolvedValueStructure.structName {
					return ctx.err("expected variable type " + resolvedValueStructure.structName + " but got " + typeName)
				}
			default:
				return ctx.err("cannot set field using variable")
			}
		}
	}

	hasDefaultValue := ctx.readInst() == 't'
	ctx.skipInst(1)

	found, criticalErr := ctx.bindExternalVariableValue(goValue, valueStructure, argumentName)
	if criticalErr {
		return criticalErr
	}
	if found {
		return false
	}

	if !hasDefaultValue {
		return ctx.err("variable has no value nor default")
	}

	return ctx.bindInputToGoValue(goValue, valueStructure, false)
}

func (ctx *Ctx) bindExternalVariableValue(goValue *reflect.Value, valueStructure *input, argumentName string) (found bool, criticalErr bool) {
	if !ctx.variablesParsed {
		if len(ctx.rawVariables) == 0 {
			return false, false
		}

		ctx.variablesParsed = true
		var err error
		ctx.variables, err = ctx.variablesJSONParser.Parse(ctx.rawVariables)
		if err != nil {
			return false, ctx.err(err.Error())
		}
		if ctx.variables.Type() != fastjson.TypeObject {
			return false, ctx.err("variables provided must be of type object")
		}
	}

	variable := ctx.variables.Get(argumentName)
	if variable == nil {
		return false, false
	}

	return true, ctx.bindJSONToValue(goValue, valueStructure, variable)
}

func (ctx *Ctx) bindJSONToValue(goValue *reflect.Value, valueStructure *input, jsonData *fastjson.Value) bool {
	jsonDataType := jsonData.Type()
	if valueStructure.isEnum || valueStructure.isID || valueStructure.isFile || valueStructure.isTime {
		if jsonDataType != fastjson.TypeString {
			if valueStructure.isEnum {
				return ctx.err("cannot assign " + jsonDataType.String() + " to Enum value")
			} else if valueStructure.isID {
				return ctx.err("cannot assign " + jsonDataType.String() + " to ID value")
			} else if valueStructure.isFile {
				return ctx.err("cannot assign " + jsonDataType.String() + " to File value")
			} else if valueStructure.isTime {
				return ctx.err("cannot assign " + jsonDataType.String() + " to Time value")
			} else {
				return ctx.err("cannot assign " + jsonDataType.String() + " to this field's value")
			}
		}
		stringValue := b2s(jsonData.GetStringBytes())

		if valueStructure.isEnum {
			if jsonDataType != fastjson.TypeString {
				return ctx.err("cannot assign " + jsonDataType.String() + " to ID value")
			}

			enum := ctx.schema.definedEnums[valueStructure.enumTypeIndex]
			for _, entry := range enum.entries {
				if entry.key == stringValue {
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

			return ctx.errf("unknown enum value %s for enum %s", stringValue, enum.typeName)
		} else if valueStructure.isID {
			if jsonDataType != fastjson.TypeString {
				return ctx.err("cannot assign " + jsonDataType.String() + " to ID value")
			}

			switch goValue.Kind() {
			case reflect.String:
				goValue.SetString(stringValue)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				intValue, err := strconv.Atoi(stringValue)
				if err != nil {
					return ctx.err("id argument must match a number type")
				}
				// TODO check if the int value can be assigned to int8 - int32
				goValue.SetInt(int64(intValue))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				intValue, err := strconv.Atoi(stringValue)
				if err != nil {
					return ctx.err("id argument must match a number type")
				}
				if intValue < 0 {
					return ctx.err("id argument must match a number above 0")
				}
				// TODO check if the int value can be assigned to uint8 - uint32
				goValue.SetUint(uint64(intValue))
			default:
				return ctx.err("internal error: cannot assign to this ID field")
			}
		} else if valueStructure.isFile {
			if jsonDataType != fastjson.TypeString {
				return ctx.err("cannot assign " + jsonDataType.String() + " to Time value")
			}

			file, err := ctx.getFormFile(stringValue)
			if err != nil {
				return ctx.err(err.Error())
			}
			goValue.Set(reflect.ValueOf(file))
		} else if valueStructure.isTime {
			if jsonDataType != fastjson.TypeString {
				return ctx.err("cannot assign " + jsonDataType.String() + " to Time value")
			}

			parsedTime, err := helpers.ParseIso8601String(stringValue)
			if err != nil {
				return ctx.err(err.Error())
			}
			goValue.Set(reflect.ValueOf(parsedTime))
		}
		return false
	}

	switch jsonDataType {
	case fastjson.TypeNull:
		// keep goValue at it's default
	case fastjson.TypeObject:
		if goValue.Kind() != reflect.Struct {
			return ctx.err("cannot assign object to non object value")
		}

		if valueStructure.isStructPointers {
			valueStructure = ctx.schema.inTypes[valueStructure.structName]
		}

		jsonObj := jsonData.GetObject()
		criticalErr := false
		jsonObj.Visit(func(key []byte, v *fastjson.Value) {
			if criticalErr {
				return
			}

			structItemMeta, ok := valueStructure.structContent[b2s(key)]
			if !ok {
				criticalErr = ctx.err("undefined property " + b2s(key))
				return
			}

			goValueField := goValue.Field(structItemMeta.goFieldIdx)
			criticalErr = ctx.bindJSONToValue(&goValueField, &structItemMeta, v)
		})
		if criticalErr {
			return criticalErr
		}
	case fastjson.TypeArray:
		if goValue.Kind() != reflect.Slice {
			return ctx.err("cannot assign slice to " + goValue.String())
		}

		variableArray := jsonData.GetArray()

		arr := reflect.MakeSlice(goValue.Type(), len(variableArray), len(variableArray))

		for i, variableArrayItem := range variableArray {
			arrEntry := arr.Index(i)
			criticalErr := ctx.bindJSONToValue(&arrEntry, valueStructure.elem, variableArrayItem)
			if criticalErr {
				return criticalErr
			}
		}

		goValue.Set(arr)
	case fastjson.TypeString:
		criticalErr := ctx.assignStringToValue(goValue, valueStructure, b2s(jsonData.GetStringBytes()))
		if criticalErr {
			return criticalErr
		}
	case fastjson.TypeNumber:
		goValueKind := goValue.Kind()
		if goValueKind == reflect.Float64 || goValueKind == reflect.Float32 {
			goValue.SetFloat(jsonData.GetFloat64())
		} else {
			intVal, err := jsonData.Int64()
			if err != nil {
				return ctx.err(err.Error())
			}
			switch goValueKind {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				switch goValue.Kind() {
				case reflect.Int8:
					if int64(int8(intVal)) != intVal {
						return ctx.errf("cannot assign %d to a 8bit integer", intVal)
					}
				case reflect.Int16:
					if int64(int16(intVal)) != intVal {
						return ctx.errf("cannot assign %d to a 16bit integer", intVal)
					}
				case reflect.Int32:
					if int64(int32(intVal)) != intVal {
						return ctx.errf("cannot assign %d to a 32bit integer", intVal)
					}
				}

				goValue.SetInt(intVal)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if intVal < 0 {
					return ctx.errf("cannot assign %d to a unsigned integer", intVal)
				}
				uintVal := uint64(intVal)

				switch goValue.Kind() {
				case reflect.Int8:
					if uint64(uint8(uintVal)) != uintVal {
						return ctx.errf("cannot assign %d to a 8bit unsigned integer", uintVal)
					}
				case reflect.Int16:
					if uint64(uint16(uintVal)) != uintVal {
						return ctx.errf("cannot assign %d to a 16bit unsigned integer", uintVal)
					}
				case reflect.Int32:
					if uint64(uint32(uintVal)) != uintVal {
						return ctx.errf("cannot assign %d to a 32bit unsigned integer", uintVal)
					}
				}

				goValue.SetUint(uintVal)
			case reflect.Bool:
				// TODO
				goValue.SetBool(intVal > 0)
			default:
				return ctx.err("cannot assign boolean to " + goValue.String())
			}
		}
	case fastjson.TypeTrue:
		if goValue.Kind() != reflect.Bool {
			return ctx.err("cannot assign boolean to " + goValue.String())
		}
		goValue.SetBool(true)
	case fastjson.TypeFalse:
		if goValue.Kind() != reflect.Bool {
			return ctx.err("cannot assign boolean to " + goValue.String())
		}
		goValue.SetBool(false)
	default:
		return ctx.err("variable value is of an unsupported type")
	}

	return false
}

func (ctx *Ctx) assignStringToValue(goValue *reflect.Value, valueStructure *input, stringValue string) bool {
	if valueStructure.isEnum {
		enum := ctx.schema.definedEnums[valueStructure.enumTypeIndex]
		for _, entry := range enum.entries {
			if entry.key == stringValue {
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

		return ctx.errf("unknown enum value %s for enum %s", stringValue, enum.typeName)
	} else if valueStructure.isID {
		switch goValue.Kind() {
		case reflect.String:
			goValue.SetString(stringValue)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intValue, err := strconv.Atoi(stringValue)
			if err != nil {
				return ctx.err("id argument must match a number type")
			}
			// TODO check if the int value can be assigned to int8 - int32
			goValue.SetInt(int64(intValue))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			intValue, err := strconv.Atoi(stringValue)
			if err != nil {
				return ctx.err("id argument must match a number type")
			}
			if intValue < 0 {
				return ctx.err("id argument must match a number above 0")
			}
			// TODO check if the int value can be assigned to uint8 - uint32
			goValue.SetUint(uint64(intValue))
		default:
			return ctx.err("internal error: cannot assign to this ID field")
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
		parsedTime, err := helpers.ParseIso8601String(stringValue)
		if err != nil {
			return ctx.err(err.Error())
		}
		goValue.Set(reflect.ValueOf(parsedTime))
	} else if goValue.Kind() == reflect.String {
		goValue.SetString(stringValue)
	} else {
		return ctx.err("cannot assign string to " + goValue.String())
	}
	return false
}

func (ctx *Ctx) bindInputToGoValue(goValue *reflect.Value, valueStructure *input, variablesAllowed bool) bool {
	// TODO convert to go value kind to graphql value kind in errors

	if goValue.Kind() == reflect.Ptr && !valueStructure.isFile {
		if ctx.query.Res[ctx.charNr+1] == bytecode.ValueNull {
			// keep goValue at it's default
			ctx.skipInst(6)
			return false
		}

		goValueElem := goValue.Type().Elem()
		newVal := reflect.New(goValueElem)
		newValElem := newVal.Elem()

		criticalErr := ctx.bindInputToGoValue(&newValElem, valueStructure.elem, variablesAllowed)
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

	ctx.skipInst(1)             // read ActionValue
	valueKind := ctx.readInst() // read value kind
	ctx.skipInst(4)             // read length of value

	// TODO if field is: isTime, isFile, is.. and the value provided is different than the expected we'll get wired errors

	switch valueKind {
	case bytecode.ValueVariable:
		if !variablesAllowed {
			return ctx.err("variables are not allowed here")
		}

		varNameStart, varNameEnd := getValue()
		varName := b2s(ctx.query.Res[varNameStart:varNameEnd])

		restorePositionTo := ctx.charNr
		foundArgument := ctx.findOperatorArgument(varName)
		if !foundArgument {
			ctx.charNr = restorePositionTo
			return ctx.err("variable " + varName + " not defined")
		}
		criticalErr := ctx.bindOperatorArgumentTo(goValue, valueStructure, varName)
		ctx.charNr = restorePositionTo
		if criticalErr {
			return criticalErr
		}
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
					return ctx.err("cannot assign " + intValue + " to a 8bit integer")
				}
			case reflect.Int16:
				if int64(int16(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 16bit integer")
				}
			case reflect.Int32:
				if int64(int32(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 32bit integer")
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
					return ctx.err("cannot assign " + intValue + " to a 8bit unsigned integer")
				}
			case reflect.Uint16:
				if uint64(uint16(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 16bit unsigned integer")
				}
			case reflect.Uint32:
				if uint64(uint32(value)) != value {
					return ctx.err("cannot assign " + intValue + " to a 32bit unsigned integer")
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
		criticalErr := ctx.assignStringToValue(goValue, valueStructure, stringValue)
		if criticalErr {
			return criticalErr
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

		enum := ctx.schema.definedEnums[valueStructure.enumTypeIndex]
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
			criticalErr := ctx.bindInputToGoValue(&arrayEntry, valueStructure.elem, variablesAllowed)
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
		ctx.skipInst(-6)

		criticalErr := ctx.walkInputObject(func(key []byte) bool {
			structFieldValueStructure, ok := valueStructure.structContent[b2s(key)]
			if !ok {
				return ctx.err("undefined property " + b2s(key))
			}

			field := goValue.Field(structFieldValueStructure.goFieldIdx)
			return ctx.bindInputToGoValue(&field, &structFieldValueStructure, variablesAllowed)
		})
		if criticalErr {
			return criticalErr
		}
	}
	return false
}

// walkInputObject walks over an input object and triggers onValueOfKey after reading a key and reached it value
// onValueOfKey is expected to parse the value before returning
func (ctx *Ctx) walkInputObject(onValueOfKey func(key []byte) bool) bool {
	// Read ActionValue and ValueObject and NULL * 5
	ctx.skipInst(7)

	for {
		// Check if the current or next value is the end
		c := ctx.readInst()
		if c == 'e' || (c == 0 && ctx.readInst() == 'e') {
			// end of object
			ctx.skipInst(1) // skip next NULL byte
			return false
		}
		keyStart := ctx.charNr
		var keyEnd int
		for {
			c = ctx.readInst()
			if c == 0 {
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

func (ctx *Ctx) valueToJson(in reflect.Value, kind reflect.Kind) {
	switch kind {
	case reflect.String:
		stringToJson(in.String(), &ctx.schema.Result)
	case reflect.Bool:
		if in.Bool() {
			ctx.write([]byte("true"))
		} else {
			ctx.write([]byte("false"))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ctx.schema.Result = strconv.AppendInt(ctx.schema.Result, in.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ctx.schema.Result = strconv.AppendUint(ctx.schema.Result, in.Uint(), 10)
	case reflect.Float32:
		floatToJson(32, in.Float(), &ctx.schema.Result)
	case reflect.Float64:
		floatToJson(64, in.Float(), &ctx.schema.Result)
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

func (ctx *Ctx) startTrace() {
	if ctx.tracingEnabled {
		ctx.prefRecordingStartTime = time.Now()
	}
}

func (ctx *Ctx) finishTrace(report func(offset, duration int64)) {
	f := ctx.prefRecordingStartTime
	offset := f.Sub(ctx.tracing.GoStartTime).Nanoseconds()
	duration := time.Since(f).Nanoseconds()
	report(offset, duration)
}

// b2s converts a byte array into a string without allocating new memory
// Note that any changes to a will result in a different string
func b2s(a []byte) string {
	return *(*string)(unsafe.Pointer(&a))
}

func s2b(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

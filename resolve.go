package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type ResolveOptions struct {
	OperatorTarget string
	Variables      string // Expects valid JSON or empty string
	Context        context.Context
	Values         map[string]interface{}                          // Passed directly to the request context
	GetFormFile    func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
	Tracing        bool                                            // https://github.com/apollographql/apollo-tracing
	ReturnOnlyData bool
}

func (s *Schema) Resolve(query string, options ResolveOptions) ([]byte, []error) {
	s.m.Lock()
	defer s.m.Unlock()

	s.ctx.schema = s

	s.ctx.Reset(
		s,
		options.GetFormFile,
		options.Context,
		options.Variables,
		options.Tracing,
	)

	s.ctx.result = s.ctx.result[:0]
	if !options.ReturnOnlyData {
		s.ctx.write([]byte(`{"data":`))
	}

	sendEmptyResult := s.ResolveContent(query, &options)
	if options.Tracing {
		s.ctx.tracing.finish()
		s.ctx.SetExtension("tracing", s.ctx.tracing)
	}
	if !options.ReturnOnlyData {
		s.ctx.CompleteResult(sendEmptyResult, true, true)
	}

	return s.ctx.result, s.ctx.errors
}

func (ctx *Ctx) Reset(
	schema *Schema,
	getFormFile func(key string) (*multipart.FileHeader, error),
	context context.Context,
	jsonVariablesString string,
	tracing bool,
) {
	*ctx = Ctx{
		schema:              schema,
		errors:              ctx.errors[:0],
		operator:            ctx.operator,
		jsonVariablesString: jsonVariablesString,
		jsonVariables:       nil,
		path:                ctx.path[:0],
		context:             context,
		getFormFile:         getFormFile,
		tracingEnabled:      tracing,
		tracing:             ctx.tracing,

		reflectValues: ctx.reflectValues,
		result:        ctx.result[:0],
		funcInputs:    ctx.funcInputs[:0],

		Values: map[string]interface{}{},
	}

	if tracing {
		ctx.tracing = &tracer{
			Version:     1,
			GoStartTime: time.Now(),
			Execution: tracerExecution{
				Resolvers: []tracerResolver{},
			},
		}
	}
}

func (ctx *Ctx) CompleteResult(sendEmptyResult, includeErrs, includeExtensions bool) {
	if sendEmptyResult {
		ctx.result = ctx.result[:0]
		ctx.write([]byte(`{"data":{}`))
	}

	if includeErrs && len(ctx.errors) > 0 {
		ctx.write([]byte(`,"errors":[`))
		for i, err := range ctx.errors {
			if i > 0 {
				ctx.writeByte(',')
			}
			ctx.write([]byte(`{"message":`))
			stringToJson([]byte(err.Error()), &ctx.result)

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
		ctx.writeByte(']')
	}

	if includeExtensions && len(ctx.extensions) > 0 {
		extensionsJson, err := json.Marshal(ctx.extensions)
		if err == nil {
			ctx.write([]byte(`,"extensions":`))
			ctx.write(extensionsJson)
		}
	}

	strings.NewReader("some io.Reader stream to be read\n")

	ctx.writeByte('}')
}

func (s *Schema) ResolveContent(query string, options *ResolveOptions) (treadResultAsEmpty bool) {
	s.ctx.startTrace()
	s.iter.parseQuery(query)
	if len(s.iter.resErrors) > 0 {
		return
	}

	if s.ctx.tracingEnabled {
		s.ctx.finishTrace(func(offset, duration int64) {
			s.ctx.tracing.Parsing.StartOffset = offset
			s.ctx.tracing.Parsing.Duration = duration
		})

		s.ctx.tracing.Validation.StartOffset = s.ctx.prefRecordingStartTime.Sub(time.Now()).Nanoseconds()
	}

	if len(s.iter.resErrors) > 0 {
		return true
	}

	if options.Values != nil {
		s.ctx.Values = options.Values
	}

	switch len(s.iter.operatorsMap) {
	case 0:
		return true
	case 1:
		for _, operator := range s.iter.operatorsMap {
			s.ctx.operator = &operator
		}
	default:
		if options.OperatorTarget == "" {
			s.ctx.errors = append(s.ctx.errors, errors.New("multiple operators defined without target"))
			return true
		}

		operator, ok := s.iter.operatorsMap[options.OperatorTarget]
		if !ok {
			operatorsList := []string{}
			for k := range s.iter.operatorsMap {
				operatorsList = append(operatorsList, k)
			}
			s.ctx.errors = append(s.ctx.errors, errors.New(options.OperatorTarget+" is not a valid operator, available operators: "+strings.Join(operatorsList, ", ")))
			return true
		}
		s.ctx.operator = &operator
	}

	s.ctx.start()
	return false
}

func (ctx *Ctx) start() {
	switch ctx.operator.operationType {
	case "query":
		ctx.reflectValues[0] = ctx.schema.rootQueryValue
		ctx.resolveSelection(ctx.operator.selectionIdx, ctx.schema.rootQuery, 0)
	case "mutation":
		ctx.reflectValues[0] = ctx.schema.rootMethodValue
		ctx.resolveSelection(ctx.operator.selectionIdx, ctx.schema.rootMethod, 0)
	case "subscription":
		// TODO
		ctx.addErr("subscription not suppored yet")
		ctx.write([]byte("{}"))
	default:
		ctx.addErrf("%s cannot be used as operator", ctx.operator.operationType)
		ctx.write([]byte("{}"))
	}
}

func (ctx *Ctx) resolveSelection(selectionIdx int, structType *obj, dept uint8) {
	if dept >= ctx.schema.MaxDepth {
		ctx.addErr("reached max dept")
		ctx.write([]byte("null"))
		return
	}

	ctx.writeByte('{')
	ctx.resolveSelectionContent(selectionIdx, structType, dept, len(ctx.result))
	ctx.writeByte('}')
}

func (ctx *Ctx) resolveSelectionContentDirectiveCheck(directives directives) (include bool, err error) {
	include = true
loop:
	for directiveName, arg := range directives {
		switch directiveName {
		case "skip", "include":
			ifArg, ok := arg.arguments["if"]
			if !ok {
				return false, errors.New("if argument missing in skip directive")
			}

			v := reflect.New(reflect.TypeOf(true)).Elem()
			err = ctx.matchInputValue(&ifArg, &v, &input{kind: reflect.Bool})
			if err != nil {
				return false, err
			}

			include = v.Bool()
			if directiveName == "skip" {
				include = !include
			}
			if !include {
				break loop
			}
		default:
			return false, fmt.Errorf("unknown directive %s", directiveName)
		}
	}
	return
}

func (ctx *Ctx) resolveSelectionContent(selectionIdx int, structType *obj, dept uint8, startLen int) {
	dept = dept + 1

	for _, selection := range ctx.schema.iter.selections[selectionIdx] {
		switch selection.selectionType {
		case "Field":
			if len(selection.field.directives) > 0 {
				include, err := ctx.resolveSelectionContentDirectiveCheck(selection.field.directives)
				if err != nil {
					ctx.addErrf(err.Error())
					continue
				}
				if !include {
					continue
				}
			}

			ctx.resolveField(selection.field, structType, dept, len(ctx.result) > startLen)
		case "FragmentSpread":
			if dept >= ctx.schema.MaxDepth {
				continue
			}

			if len(selection.fragmentSpread.directives) > 0 {
				include, err := ctx.resolveSelectionContentDirectiveCheck(selection.fragmentSpread.directives)
				if err != nil {
					ctx.addErrf(err.Error())
					continue
				}
				if !include {
					continue
				}
			}

			operator, ok := ctx.schema.iter.fragments[selection.fragmentSpread.name]
			if !ok {
				ctx.addErrf("unknown fragment %s", selection.fragmentSpread.name)
				continue
			}

			ctx.resolveSelectionContent(operator.fragment.selectionIdx, structType, dept, startLen)
		case "InlineFragment":
			if dept >= ctx.schema.MaxDepth {
				continue
			}

			if len(selection.inlineFragment.directives) > 0 {
				include, err := ctx.resolveSelectionContentDirectiveCheck(selection.inlineFragment.directives)
				if err != nil {
					ctx.addErrf(err.Error())
					continue
				}
				if !include {
					continue
				}
			}

			ctx.resolveSelectionContent(selection.inlineFragment.selectionIdx, structType, dept, startLen)
		}
	}
}

func (ctx *Ctx) resolveField(query *field, codeStructure *obj, dept uint8, placeCommaInFront bool) {
	ctx.startTrace()

	if query.name == "__typename" {
		// TODO currently this isn't traced
		if placeCommaInFront {
			ctx.writeByte(',')
		}
		ctx.writeByte('"')
		if len(query.alias) > 0 {
			ctx.write(query.alias)
		} else {
			ctx.writeString(query.name)
		}
		ctx.write([]byte(`":"`))
		ctx.write(codeStructure.typeNameBytes)
		ctx.writeByte('"')
		return
	}

	prefPathLen := len(ctx.path)
	ctx.path = append(ctx.path, []byte(`,"`)...)
	// Note that depending of the result below the path might be appended differently
	// This is to avoid converting a string to bytes in almost all queries

	structItem, ok := codeStructure.objContents[query.name]
	if !ok {
		if len(query.alias) > 0 {
			ctx.path = append(ctx.path, query.alias...)
		} else {
			ctx.path = append(ctx.path, []byte(query.name)...)
		}
		ctx.path = append(ctx.path, '"')

		ctx.addErrf("%s does not exists on %s", query.name, codeStructure.typeName)
		return
	}

	name := structItem.qlFieldName
	if len(query.alias) > 0 {
		name = query.alias
	}

	ctx.path = append(ctx.path, name...)
	ctx.path = append(ctx.path, '"')

	defer ctx.finishTrace(func(offset, duration int64) {
		returnType := bytes.NewBuffer(nil)
		ctx.schema.objToQlTypeName(structItem, returnType)

		ctx.tracing.Execution.Resolvers = append(ctx.tracing.Execution.Resolvers, tracerResolver{
			Path:        json.RawMessage(ctx.Path()),
			ParentType:  codeStructure.typeName,
			FieldName:   query.name,
			ReturnType:  returnType.String(),
			StartOffset: offset,
			Duration:    duration,
		})
	})

	if placeCommaInFront {
		ctx.writeByte(',')
	}
	ctx.writeByte('"')
	ctx.write(name)
	ctx.write([]byte(`":`))

	value := ctx.value()
	ctx.currentReflectValueIdx++
	if structItem.customObjValue != nil {
		ctx.reflectValues[ctx.currentReflectValueIdx] = *structItem.customObjValue
	} else if structItem.valueType == valueTypeMethod && structItem.method.isTypeMethod {
		ctx.reflectValues[ctx.currentReflectValueIdx] = value.Method(structItem.structFieldIdx)
	} else {
		ctx.reflectValues[ctx.currentReflectValueIdx] = value.Field(structItem.structFieldIdx)
	}

	ctx.resolveFieldDataValue(query, structItem, dept)
	ctx.path = ctx.path[:prefPathLen]
	ctx.currentReflectValueIdx--
}

func (ctx *Ctx) matchInputValue(queryValue *value, goField *reflect.Value, goAnalyzedData *input) error {

	if goAnalyzedData.isFile {
		goAnalyzedData.kind = reflect.String
		if queryValue.isNull {
			return nil
		}
	}

	if goAnalyzedData.kind == reflect.Ptr {
		if queryValue.isNull {
			// Na mate just keep it at it's default
			return nil
		}

		expectedType := goField.Type().Elem()
		newVal := reflect.New(expectedType)
		newValInner := newVal.Elem()

		err := ctx.matchInputValue(queryValue, &newValInner, goAnalyzedData.elem)
		if err != nil {
			return err
		}

		goField.Set(newVal)
		return nil
	}

	mismatchError := func() error {
		m := map[reflect.Kind]string{
			reflect.Invalid:       "an unknown type",
			reflect.Bool:          "a Boolean",
			reflect.Int:           "a number",
			reflect.Int8:          "a number",
			reflect.Int16:         "a number",
			reflect.Int32:         "a number",
			reflect.Int64:         "a number",
			reflect.Uint:          "a number",
			reflect.Uint8:         "a number",
			reflect.Uint16:        "a number",
			reflect.Uint32:        "a number",
			reflect.Uint64:        "a number",
			reflect.Uintptr:       "a number",
			reflect.Float32:       "a Float",
			reflect.Float64:       "a Float",
			reflect.Complex64:     "a number",
			reflect.Complex128:    "a number",
			reflect.Array:         "an array",
			reflect.Chan:          "an unknown type",
			reflect.Func:          "an unknown type",
			reflect.Interface:     "an unknown type",
			reflect.Map:           "an unknown type",
			reflect.Ptr:           "optional type",
			reflect.Slice:         "an array",
			reflect.String:        "a String",
			reflect.Struct:        "a object",
			reflect.UnsafePointer: "a number",
		}

		if goAnalyzedData.isFile {
			return errors.New("arguments type missmatch expected a string pointing to a form file")
		}
		if goAnalyzedData.isTime {
			return errors.New("argument type missmatch expected a string in ISO 8601 format")
		}
		return fmt.Errorf("arguments type missmatch expected %s", m[goField.Type().Kind()])
	}

	if queryValue.isVar {
		err := ctx.getVariable(queryValue.variable, queryValue)
		if err != nil {
			return err
		}
	}

	if queryValue.isNull {
		// Na mate just keep it at it's default
		return nil
	}

	setString := func(str string) error {
		if goAnalyzedData.isTime {
			parsedTime, err := parseTime(str)
			if err != nil {
				return err
			}
			goField.Set(reflect.ValueOf(parsedTime))
		} else if goAnalyzedData.isFile {
			file, err := ctx.getFormFile(str)
			if err != nil {
				return err
			}

			goField.Set(reflect.ValueOf(file))
		} else {
			goField.SetString(str)
		}
		return nil
	}

	if queryValue.isEnum {
		if !goAnalyzedData.isEnum {
			return mismatchError()
		}

		enum := definedEnums[goAnalyzedData.enumTypeIndex]
		if queryValue.qlTypeName != nil && *queryValue.qlTypeName != enum.typeName {
			return fmt.Errorf("expected type %s but got %s", enum.typeName, *queryValue.qlTypeName)
		}

		switch enum.contentKind {
		case reflect.String:
			for _, entry := range enum.entries {
				if entry.key == queryValue.enumValue {
					setString(entry.value.String())
					return nil
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			for _, entry := range enum.entries {
				if entry.key == queryValue.enumValue {
					goField.SetInt(entry.value.Int())
					return nil
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			for _, entry := range enum.entries {
				if entry.key == queryValue.enumValue {
					goField.SetUint(entry.value.Uint())
					return nil
				}
			}
		default:
			return errors.New("internal error, type missmatch on enum")
		}

		return fmt.Errorf("unknown enum value %s for enum %s", queryValue.enumValue, enum.typeName)
	} else {
		switch queryValue.valType {
		case reflect.Int:
			switch goAnalyzedData.kind {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				goField.SetInt(int64(queryValue.intValue))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if queryValue.intValue < 0 {
					return errors.New("argument cannot be less than 0")
				}
				goField.SetUint(uint64(queryValue.intValue))
			case reflect.Float32, reflect.Float64:
				goField.SetFloat(float64(queryValue.intValue))
			default:
				return mismatchError()
			}
		case reflect.Float64:
			switch goAnalyzedData.kind {
			case reflect.Float32, reflect.Float64:
				goField.SetFloat(queryValue.floatValue)
			default:
				return mismatchError()
			}
		case reflect.String:
			if goAnalyzedData.kind == reflect.String {
				setString(queryValue.stringValue)
			} else if goAnalyzedData.isID {
				switch goAnalyzedData.kind {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					intValue, err := strconv.Atoi(queryValue.stringValue)
					if err != nil {
						return errors.New("id argument must match a number type")
					}
					goField.SetInt(int64(intValue))
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					intValue, err := strconv.Atoi(queryValue.stringValue)
					if err != nil {
						return errors.New("id argument must match a number type")
					}
					if intValue < 0 {
						return errors.New("id argument must match a number above 0")
					}
					goField.SetUint(uint64(intValue))
				default:
					return mismatchError()
				}
			} else {
				return mismatchError()
			}
		case reflect.Bool:
			if goAnalyzedData.kind == reflect.Bool {
				goField.SetBool(queryValue.booleanValue)
			} else {
				return mismatchError()
			}
		case reflect.Array:
			if goAnalyzedData.kind == reflect.Array {
				// TODO support this
				return errors.New("fixed length arrays not supported")
			}

			if goAnalyzedData.kind != reflect.Slice {
				return mismatchError()
			}

			arr := reflect.MakeSlice(goField.Type(), len(queryValue.listValue), len(queryValue.listValue))

			for i, item := range queryValue.listValue {
				arrayItem := arr.Index(i)
				err := ctx.matchInputValue(&item, &arrayItem, goAnalyzedData.elem)
				if err != nil {
					return fmt.Errorf("%s, Array index: [%d]", err.Error(), i)
				}
			}

			goField.Set(arr)
		case reflect.Map:
			if goAnalyzedData.kind != reflect.Struct {
				return mismatchError()
			}

			if queryValue.qlTypeName != nil && *queryValue.qlTypeName != goAnalyzedData.structName {
				return fmt.Errorf("expected type %s but got %s", goAnalyzedData.structName, *queryValue.qlTypeName)
			}

			if goAnalyzedData.isStructPointers {
				goAnalyzedData = ctx.schema.inTypes[goAnalyzedData.structName]
			}

			for queryKey, arg := range queryValue.objectValue {
				structItemMeta, ok := goAnalyzedData.structContent[queryKey]
				if !ok {
					return fmt.Errorf("undefined property %s", queryKey)
				}

				field := goField.Field(structItemMeta.goFieldIdx)
				err := ctx.matchInputValue(&arg, &field, &structItemMeta)
				if err != nil {
					return fmt.Errorf("%s, property: %s", err.Error(), queryKey)
				}
			}
		default:
			return errors.New("undefined function input type")
		}
	}

	return nil
}

func (ctx *Ctx) resolveFieldDataValue(query *field, codeStructure *obj, dept uint8) {
	value := ctx.value()
	switch codeStructure.valueType {
	case valueTypeMethod:
		method := codeStructure.method

		if !method.isTypeMethod && value.IsNil() {
			ctx.write([]byte("null"))
			return
		}

		ctx.funcInputs = ctx.funcInputs[:0]
		for _, in := range method.ins {
			if in.isCtx {
				ctx.funcInputs = append(ctx.funcInputs, reflect.ValueOf(ctx))
			} else {
				ctx.funcInputs = append(ctx.funcInputs, reflect.New(*in.type_).Elem())
			}
		}

		for queryKey, queryValue := range query.arguments {
			inField, ok := method.inFields[queryKey]
			if !ok {
				ctx.addErrf("undefined input: %s", queryKey)
				continue
			}
			goField := ctx.funcInputs[inField.inputIdx].Field(inField.input.goFieldIdx)

			err := ctx.matchInputValue(&queryValue, &goField, &inField.input)
			if err != nil {
				ctx.addErrf("%s, property: %s", err.Error(), queryKey)
				ctx.write([]byte("null"))
				return
			}
		}

		outs := value.Call(ctx.funcInputs)

		if method.errorOutNr != nil {
			errOut := outs[*method.errorOutNr]
			if !errOut.IsNil() {
				err, ok := errOut.Interface().(error)
				if !ok {
					ctx.addErr("returned a invalid kind of error")
					ctx.write([]byte("null"))
					return
				} else if err != nil {
					ctx.addErr(err.Error())
				}
			}
		}

		if ctx.context != nil {
			err := ctx.context.Err()
			if err != nil {
				// Context ended
				ctx.addErr(err.Error())
				ctx.write([]byte("null"))
				return
			}
		}

		ctx.currentReflectValueIdx++
		ctx.reflectValues[ctx.currentReflectValueIdx] = outs[method.outNr]
		ctx.resolveFieldDataValue(query, &method.outType, dept)
		ctx.currentReflectValueIdx--
		return
	case valueTypeArray:
		if (value.Kind() != reflect.Array && value.Kind() != reflect.Slice) || value.IsNil() {
			ctx.write([]byte("null"))
			return
		}

		if codeStructure.innerContent == nil {
			ctx.addErr("server didn't expected an array")
			ctx.write([]byte("null"))
			return
		}
		codeStructure = codeStructure.innerContent

		ctx.writeByte('[')
		for i := 0; i < value.Len(); i++ {
			prefPathLen := len(ctx.path)
			ctx.path = append(ctx.path, ',')
			ctx.path = strconv.AppendInt(ctx.path, int64(i), 10)

			ctx.currentReflectValueIdx++
			ctx.reflectValues[ctx.currentReflectValueIdx] = value.Index(i)

			ctx.resolveFieldDataValue(query, codeStructure, dept)
			if i < value.Len()-1 {
				ctx.writeByte(',')
			}

			ctx.path = ctx.path[:prefPathLen]
			ctx.currentReflectValueIdx--
		}
		ctx.writeByte(']')
		return
	case valueTypeObj, valueTypeObjRef:
		if query.selectionIdx == -1 || len(ctx.schema.iter.selections[query.selectionIdx]) == 0 {
			ctx.addErr("must have a selection")
			ctx.write([]byte("null"))
			return
		}

		var ok bool
		if codeStructure.valueType == valueTypeObjRef {
			codeStructure, ok = ctx.schema.types[codeStructure.typeName]
			if !ok {
				ctx.addErr("cannot have a selection")
				ctx.write([]byte("null"))
				return
			}
		}

		ctx.resolveSelection(query.selectionIdx, codeStructure, dept)
		return
	case valueTypeData:
		if query.selectionIdx >= 0 && len(ctx.schema.iter.selections[query.selectionIdx]) != 0 {
			ctx.addErr("cannot have a selection")
			ctx.write([]byte("null"))
			return
		}

		if codeStructure.isID && codeStructure.dataValueType != reflect.String {
			// Graphql ID fields are always strings
			ctx.writeByte('"')
			ctx.valueToJson(value, codeStructure.dataValueType)
			ctx.writeByte('"')
		} else {
			ctx.valueToJson(value, codeStructure.dataValueType)
		}

		return
	case valueTypePtr:
		if value.Kind() != reflect.Ptr || value.IsNil() {
			ctx.write([]byte("null"))
			return
		}

		ctx.currentReflectValueIdx++
		ctx.reflectValues[ctx.currentReflectValueIdx] = value.Elem()
		ctx.resolveFieldDataValue(query, codeStructure.innerContent, dept)
		ctx.currentReflectValueIdx--
		return
	case valueTypeEnum:
		enum := definedEnums[codeStructure.enumTypeIndex]
		switch enum.contentKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			underlayingValue := value.Int()
			for _, entry := range enum.entries {
				if entry.value.Int() == underlayingValue {
					ctx.writeByte('"')
					ctx.write(entry.keyBytes)
					ctx.writeByte('"')
					return
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			underlayingValue := value.Uint()
			for _, entry := range enum.entries {
				if entry.value.Uint() == underlayingValue {
					ctx.writeByte('"')
					ctx.write(entry.keyBytes)
					ctx.writeByte('"')
					return
				}
			}
		case reflect.String:
			underlayingValue := value.String()
			for _, entry := range enum.entries {
				if entry.value.String() == underlayingValue {
					ctx.writeByte('"')
					ctx.write(entry.keyBytes)
					ctx.writeByte('"')
					return
				}
			}
		}

		ctx.write([]byte(`null`))
		return
	case valueTypeTime:
		timeValue, ok := value.Interface().(time.Time)
		if !ok {
			ctx.write([]byte("null"))
			return
		}
		ctx.writeByte('"')
		timeToString(&ctx.result, timeValue)
		ctx.writeByte('"')
		return
	default:
		ctx.addErr("has invalid data type")
		ctx.write([]byte("null"))
		return
	}
}

func (ctx *Ctx) valueToJson(in reflect.Value, kind reflect.Kind) {
	switch kind {
	case reflect.String:
		stringToJson([]byte(in.String()), &ctx.result)
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

func parseTime(val string) (time.Time, error) {
	// Parse to ISO 8601
	// The ISO 8601 layout might also be "2006-01-02T15:04:05.999Z" but it's mentioned less than the current so i presume what we're now using is correct
	parsedTime, err := time.Parse("2006-01-02T15:04:05.000Z", val)
	if err != nil {
		return time.Time{}, errors.New("time value doesn't match the ISO 8601 layout")
	}
	return parsedTime, nil
}

func timeToString(target *[]byte, t time.Time) {
	*target = t.AppendFormat(*target, "2006-01-02T15:04:05.000Z")
}

func (s *Schema) objToQlTypeName(item *obj, target *bytes.Buffer) {
	suffix := []byte{}

	qlType := wrapQLTypeInNonNull(s.objToQLType(item))

	for {
		switch qlType.Kind {
		case typeKindList:
			target.WriteByte('[')
			suffix = append(suffix, ']')
		case typeKindNonNull:
			suffix = append(suffix, '!')
		default:
			if qlType.Name != nil {
				target.WriteString(*qlType.Name)
			} else {
				target.Write([]byte("Unknown"))
			}
			if len(suffix) > 0 {
				target.Write(suffix)
			}
			return
		}
		qlType = qlType.OfType
	}
}

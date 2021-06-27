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

type pathT []string

func (p pathT) toJson(target *[]byte) {
	*target = append(*target, '[')
	*target = append(*target, []byte(strings.Join(p, ","))...)
	*target = append(*target, ']')
}

func (p pathT) copy() pathT {
	if p == nil {
		return nil
	}
	res := make(pathT, len(p))
	copy(res, p)
	return res
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
		s.ctx.extensions["tracing"] = s.ctx.tracing
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
		fragments:           ctx.fragments,
		schema:              schema,
		errors:              ctx.errors[:0],
		operator:            ctx.operator,
		jsonVariablesString: jsonVariablesString,
		jsonVariables:       nil,
		path:                ctx.path[:0],
		context:             context,
		getFormFile:         getFormFile,
		extensions:          map[string]interface{}{},
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
				ctx.write([]byte(`,"path":`))
				errWPath.path.toJson(&ctx.result)
			}
			errWLocation, isErrWLocation := err.(ErrorWLocation)
			if isErrWLocation {
				ctx.write([]byte(`,"locations":[{"line":`))
				ctx.writeString(strconv.FormatUint(uint64(errWLocation.line), 10))
				ctx.write([]byte(`,"column":`))
				ctx.writeString(strconv.FormatUint(uint64(errWLocation.column), 10))
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
			ctx.writeString(string(extensionsJson))
		}
	}

	strings.NewReader("some io.Reader stream to be read\n")

	ctx.writeByte('}')
}

func (s *Schema) ResolveContent(query string, options *ResolveOptions) (treadResultAsEmpty bool) {
	fragments, operatorsMap, errs := ParseQueryAndCheckNames(query, &s.ctx)
	if len(errs) > 0 {
		return true
	}

	s.ctx.fragments = fragments
	if options.Values != nil {
		s.ctx.Values = options.Values
	}

	switch len(operatorsMap) {
	case 0:
		return true
	case 1:
		for _, operator := range operatorsMap {
			s.ctx.operator = &operator
		}
	default:
		if options.OperatorTarget == "" {
			s.ctx.errors = append(s.ctx.errors, errors.New("multiple operators defined without target"))
			return true
		}

		operator, ok := operatorsMap[options.OperatorTarget]
		if !ok {
			operatorsList := []string{}
			for k := range operatorsMap {
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
		ctx.resolveSelection(ctx.operator.selection, ctx.schema.rootQuery, 0)
	case "mutation":
		ctx.reflectValues[0] = ctx.schema.rootMethodValue
		ctx.resolveSelection(ctx.operator.selection, ctx.schema.rootMethod, 0)
	case "subscription":
		// TODO
		ctx.addErr("subscription not suppored yet")
		ctx.write([]byte("{}"))
	default:
		ctx.addErrf("%s cannot be used as operator", ctx.operator.operationType)
		ctx.write([]byte("{}"))
	}
}

func (ctx *Ctx) resolveSelection(selectionSet selectionSet, structType *obj, dept uint8) {
	if dept >= ctx.schema.MaxDepth {
		ctx.addErr("reached max dept")
		ctx.write([]byte("null"))
		return
	}

	ctx.writeByte('{')
	ctx.resolveSelectionContent(selectionSet, structType, dept, len(ctx.result))
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

func (ctx *Ctx) resolveSelectionContent(selectionSet selectionSet, structType *obj, dept uint8, startLen int) {
	dept = dept + 1

	for _, selection := range selectionSet {
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

			operator, ok := ctx.fragments[selection.fragmentSpread.name]
			if !ok {
				ctx.addErrf("unknown fragment %s", selection.fragmentSpread.name)
				continue
			}

			ctx.resolveSelectionContent(operator.fragment.selection, structType, dept, startLen)
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

			ctx.resolveSelectionContent(selection.inlineFragment.selection, structType, dept, startLen)
		}
	}
}

func (ctx *Ctx) resolveField(query *field, codeStructure *obj, dept uint8, placeCommaInFront bool) {
	ctx.startTrace()
	name := query.name
	if len(query.alias) > 0 {
		name = query.alias
	}

	if query.name == "__typename" {
		// TODO currently this isn't traced
		if placeCommaInFront {
			ctx.writeByte(',')
		}
		ctx.writeByte('"')
		ctx.writeString(name)
		ctx.write([]byte(`":"`))
		ctx.writeString(codeStructure.typeName)
		ctx.writeByte('"')
		return
	}

	pathAppend := []byte{}
	stringToJson([]byte(name), &pathAppend)
	ctx.path = append(ctx.path, string(pathAppend))

	structItem, ok := codeStructure.objContents[query.name]
	if !ok {
		ctx.addErrf("%s does not exists on %s", query.name, codeStructure.typeName)
		ctx.path = ctx.path[:len(ctx.path)-1]
		return
	}

	defer ctx.finishTrace(func(offset, duration int64) {
		path := []byte{}
		ctx.path.toJson(&path)

		returnType := bytes.NewBuffer(nil)
		ctx.schema.objToQlTypeName(structItem, returnType)

		ctx.tracing.Execution.Resolvers = append(ctx.tracing.Execution.Resolvers, tracerResolver{
			Path:        json.RawMessage(string(path)),
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
	ctx.writeString(name)
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
	ctx.path = ctx.path[:len(ctx.path)-1]
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

		if queryValue.qlTypeName != nil && *queryValue.qlTypeName != goAnalyzedData.enumTypeName {
			return fmt.Errorf("expected type %s but got %s", goAnalyzedData.enumTypeName, *queryValue.qlTypeName)
		}

		enum := definedEnums[goAnalyzedData.enumTypeName]
		value, ok := enum.keyValue[queryValue.enumValue]
		if !ok {
			return fmt.Errorf("unknown enum value %s for enum %s", queryValue.enumValue, goAnalyzedData.enumTypeName)
		}

		switch value.Kind() {
		case reflect.String:
			setString(value.String())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			goField.SetInt(value.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			goField.SetUint(value.Uint())
		default:
			return errors.New("internal error, type missmatch on enum")
		}
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
			ctx.path = append(ctx.path, strconv.Itoa(i))
			ctx.currentReflectValueIdx++
			ctx.reflectValues[ctx.currentReflectValueIdx] = value.Index(i)

			ctx.resolveFieldDataValue(query, codeStructure, dept)
			if i < value.Len()-1 {
				ctx.writeByte(',')
			}

			ctx.path = ctx.path[:len(ctx.path)-1]
			ctx.currentReflectValueIdx--
		}
		ctx.writeByte(']')
		return
	case valueTypeObj, valueTypeObjRef:
		if len(query.selection) == 0 {
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

		ctx.resolveSelection(query.selection, codeStructure, dept)
		return
	case valueTypeData:
		if len(query.selection) > 0 {
			ctx.addErr("cannot have a selection")
			ctx.write([]byte("null"))
			return
		}

		if codeStructure.isID {
			// Graphql ID fields are always strings
			if codeStructure.dataValueType == reflect.String {
				ctx.valueToJson(value.String())
			} else {
				ctx.writeByte('"')
				ctx.valueToJson(value.Interface())
				ctx.writeByte('"')
			}
		} else {
			ctx.valueToJson(value.Interface())
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
		enum := definedEnums[codeStructure.enumTypeName]

		key := enum.valueKey.MapIndex(value)
		if !key.IsValid() {
			ctx.write([]byte("null"))
			return
		}
		ctx.writeByte('"')
		ctx.writeString(key.String())
		ctx.writeByte('"')
		return
	case valueTypeTime:
		timeValue, ok := value.Interface().(time.Time)
		if !ok {
			ctx.write([]byte("null"))
			return
		}
		ctx.valueToJson(timeToString(timeValue))
		return
	default:
		ctx.addErr("has invalid data type")
		ctx.write([]byte("null"))
		return
	}
}

func (ctx *Ctx) valueToJson(in interface{}) {
	switch v := in.(type) {
	case string:
		stringToJson([]byte(v), &ctx.result)
	case bool:
		if v {
			ctx.write([]byte("true"))
		} else {
			ctx.write([]byte("false"))
		}
	case int:
		ctx.writeString(strconv.Itoa(v))
	case int8:
		ctx.writeString(strconv.Itoa(int(v)))
	case int16:
		ctx.writeString(strconv.Itoa(int(v)))
	case int32: // == rune
		ctx.writeString(strconv.Itoa(int(v)))
	case int64:
		ctx.writeString(strconv.FormatInt(v, 10))
	case uint:
		ctx.writeString(strconv.FormatUint(uint64(v), 10))
	case uint8: // == byte
		ctx.writeString(strconv.FormatUint(uint64(v), 10))
	case uint16:
		ctx.writeString(strconv.FormatUint(uint64(v), 10))
	case uint32:
		ctx.writeString(strconv.FormatUint(uint64(v), 10))
	case uint64:
		ctx.writeString(strconv.FormatUint(v, 10))
	case uintptr:
		ctx.writeString(strconv.FormatUint(uint64(v), 10))
	case float32:
		floatToJson(32, float64(v), &ctx.result)
	case float64:
		floatToJson(64, v, &ctx.result)
	case *string:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *bool:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *int:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *int8:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *int16:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *int32: // = *rune
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *int64:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *uint:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *uint8: // = *byte
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *uint16:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *uint32:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *uint64:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *uintptr:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *float32:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
		}
	case *float64:
		if v == nil {
			ctx.write([]byte("null"))
		} else {
			ctx.valueToJson(*v)
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

func timeToString(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.000Z")
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

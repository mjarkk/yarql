package graphql

import (
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
}

type pathT []string

func (p pathT) toJson() string {
	return "[" + strings.Join(p, ",") + "]"
}

func (p pathT) copy() pathT {
	if p == nil {
		return nil
	}
	res := make(pathT, len(p))
	copy(res, p)
	return res
}

func (s *Schema) Resolve(query string, options ResolveOptions) (data string, extensions map[string]interface{}, errs []error) {
	s.m.Lock()
	defer s.m.Unlock()

	s.tracingEnabled = options.Tracing
	if options.Tracing {
		s.tracing = newTracer()
	}

	s.ctx = Ctx{
		schema:              s,
		Values:              map[string]interface{}{},
		directvies:          []directives{},
		errors:              []error{},
		jsonVariablesString: options.Variables,
		context:             options.Context,
		getFormFile:         options.GetFormFile,
		extensions:          map[string]interface{}{},
	}

	fragments, operatorsMap, errs := ParseQueryAndCheckNames(query, &s.ctx)
	if len(errs) > 0 {
		return "{}", nil, errs
	}

	s.ctx.fragments = fragments
	if options.Values != nil {
		s.ctx.Values = options.Values
	}

	getExtensions := func() map[string]interface{} {
		if options.Tracing {
			s.ctx.extensions["tracing"] = s.tracing.finish()
		}
		return s.ctx.extensions
	}

	switch len(operatorsMap) {
	case 0:
		return "{}", getExtensions(), nil
	case 1:
		for _, operator := range operatorsMap {
			s.ctx.operator = &operator
		}
	default:
		if options.OperatorTarget == "" {
			return "{}", getExtensions(), []error{errors.New("multiple operators defined without target")}
		}

		operator, ok := operatorsMap[options.OperatorTarget]
		if !ok {
			operatorsList := []string{}
			for k := range operatorsMap {
				operatorsList = append(operatorsList, k)
			}
			return "{}", getExtensions(), []error{fmt.Errorf("%s is not a valid operator, available operators: %s", options.OperatorTarget, strings.Join(operatorsList, ", "))}
		}
		s.ctx.operator = &operator
	}

	res := s.ctx.start()
	return res, getExtensions(), s.ctx.errors
}

func (ctx *Ctx) start() string {
	if ctx.operator.directives != nil && len(ctx.operator.directives) > 0 {
		ctx.directvies = append(ctx.directvies, ctx.operator.directives)
	}

	ctx.path = ctx.path[:0]
	switch ctx.operator.operationType {
	case "query":
		return ctx.resolveSelection(ctx.operator.selection, ctx.schema.rootQueryValue, ctx.schema.rootQuery, 0)
	case "mutation":
		return ctx.resolveSelection(ctx.operator.selection, ctx.schema.rootMethodValue, ctx.schema.rootMethod, 0)
	case "subscription":
		// TODO
		ctx.addErr("subscription not suppored yet")
		return "{}"
	default:
		ctx.addErrf("%s cannot be used as operator", ctx.operator.operationType)
		return "{}"
	}
}

func (ctx *Ctx) resolveSelection(selectionSet selectionSet, struct_ reflect.Value, structType *obj, dept uint8) string {
	if dept >= ctx.schema.MaxDepth {
		ctx.addErr("reached max dept")
		return "null"
	}
	dept = dept + 1
	return "{" + ctx.resolveSelectionContent(selectionSet, struct_, structType, dept) + "}"
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

func (ctx *Ctx) resolveSelectionContent(selectionSet selectionSet, struct_ reflect.Value, structType *obj, dept uint8) string {
	res := ""
	writtenToRes := false
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

			value, hasError := ctx.resolveField(selection.field, struct_, structType, dept)
			if !hasError {
				if writtenToRes {
					res += ","
				} else {
					writtenToRes = true
				}
				res += value
			}
		case "FragmentSpread":
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

			value := ctx.resolveSelectionContent(operator.fragment.selection, struct_, structType, dept)
			if len(value) > 0 {
				if writtenToRes {
					res += ","
				} else {
					writtenToRes = true
				}
				res += value
			}
		case "InlineFragment":
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

			value := ctx.resolveSelectionContent(selection.inlineFragment.selection, struct_, structType, dept)
			if len(value) > 0 {
				if writtenToRes {
					res += ","
				} else {
					writtenToRes = true
				}
				res += value
			}
		}
	}
	return res
}

func (ctx *Ctx) resolveField(query *field, struct_ reflect.Value, codeStructure *obj, dept uint8) (fieldValue string, returnedOnError bool) {
	ctx.startTrace()
	name := query.name
	if len(query.alias) > 0 {
		name = query.alias
	}

	res := func(data string) string {
		return fmt.Sprintf(`"%s":%s`, name, data)
	}

	if query.name == "__typename" {
		return res(`"` + codeStructure.typeName + `"`), false
	}

	ctx.path = append(ctx.path, fmt.Sprintf("%q", name))

	structItem, ok := codeStructure.objContents[query.name]
	var value reflect.Value
	if !ok {
		ctx.addErrf("%s does not exists on %s", query.name, codeStructure.typeName)
		ctx.path = ctx.path[:len(ctx.path)-1]
		return res("null"), true
	}

	defer ctx.finishTrace(func(t *tracer, offset, duration int64) {
		t.Execution.Resolvers = append(t.Execution.Resolvers, tracerResolver{
			Path:        json.RawMessage(ctx.path.toJson()),
			ParentType:  codeStructure.typeName,
			FieldName:   query.name,
			ReturnType:  ctx.schema.objToQlTypeName(structItem),
			StartOffset: offset,
			Duration:    duration,
		})
	})

	if structItem.customObjValue != nil {
		value = *structItem.customObjValue
	} else if structItem.valueType == valueTypeMethod && structItem.method.isTypeMethod {
		value = struct_.MethodByName(structItem.structFieldName)
	} else {
		value = struct_.FieldByName(structItem.structFieldName)
	}

	fieldValue, returnedOnError = ctx.resolveFieldDataValue(query, value, structItem, dept)
	ctx.path = ctx.path[:len(ctx.path)-1]
	return res(fieldValue), returnedOnError
}

func (ctx *Ctx) matchInputValue(queryValue *value, goField *reflect.Value, goAnalyzedData *input) error {
	goFieldKind := goAnalyzedData.kind

	if goAnalyzedData.isFile {
		goAnalyzedData.kind = reflect.String
		goFieldKind = goAnalyzedData.kind
		if queryValue.isNull {
			return nil
		}
	}

	if goFieldKind == reflect.Ptr {
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
			switch goFieldKind {
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
			switch goFieldKind {
			case reflect.Float32, reflect.Float64:
				goField.SetFloat(queryValue.floatValue)
			default:
				return mismatchError()
			}
		case reflect.String:
			if goFieldKind == reflect.String {
				setString(queryValue.stringValue)
			} else if goAnalyzedData.isID {
				switch goFieldKind {
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
			if goFieldKind == reflect.Bool {
				goField.SetBool(queryValue.booleanValue)
			} else {
				return mismatchError()
			}
		case reflect.Array:
			if goFieldKind == reflect.Array {
				// TODO support this
				return errors.New("fixed length arrays not supported")
			}

			if goFieldKind != reflect.Slice {
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
			if goFieldKind != reflect.Struct {
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

				field := goField.FieldByName(structItemMeta.goFieldName)
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

func (ctx *Ctx) resolveFieldDataValue(query *field, value reflect.Value, codeStructure *obj, dept uint8) (fieldValue string, returnedOnError bool) {
	switch codeStructure.valueType {
	case valueTypeMethod:
		method := codeStructure.method

		if !method.isTypeMethod && value.IsNil() {
			return "null", false
		}

		inputs := []reflect.Value{}
		for _, in := range method.ins {
			if in.isCtx {
				inputs = append(inputs, reflect.ValueOf(ctx))
			} else {
				inputs = append(inputs, reflect.New(*in.type_).Elem())
			}
		}

		for queryKey, queryValue := range query.arguments {
			inField, ok := method.inFields[queryKey]
			if !ok {
				ctx.addErrf("undefined input: %s", queryKey)
				continue
			}
			goField := inputs[inField.inputIdx].FieldByName(inField.input.goFieldName)

			err := ctx.matchInputValue(&queryValue, &goField, &inField.input)
			if err != nil {
				ctx.addErrf("%s, property: %s", err.Error(), queryKey)
				return "null", true
			}
		}

		outs := value.Call(inputs)

		if method.errorOutNr != nil {
			errOut := outs[*method.errorOutNr]
			if !errOut.IsNil() {
				err, ok := errOut.Interface().(error)
				if !ok {
					ctx.addErr("returned a invalid kind of error")
					return "null", true
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
				return "null", true
			}
		}

		return ctx.resolveFieldDataValue(query, outs[method.outNr], &method.outType, dept)
	case valueTypeArray:
		if (value.Kind() != reflect.Array && value.Kind() != reflect.Slice) || value.IsNil() {
			return "null", false
		}

		if codeStructure.innerContent == nil {
			ctx.addErr("server didn't expected an array")
			return "null", true
		}
		codeStructure = codeStructure.innerContent

		list := []string{}
		for i := 0; i < value.Len(); i++ {
			ctx.path = append(ctx.path, fmt.Sprintf("%d", i))
			res, _ := ctx.resolveFieldDataValue(query, value.Index(i), codeStructure, dept)
			ctx.path = ctx.path[:len(ctx.path)-1]
			list = append(list, res)
		}
		return fmt.Sprintf("[%s]", strings.Join(list, ",")), false
	case valueTypeObj, valueTypeObjRef:
		if len(query.selection) == 0 {
			ctx.addErr("must have a selection")
			return "null", true
		}

		var ok bool
		if codeStructure.valueType == valueTypeObjRef {
			codeStructure, ok = ctx.schema.types[codeStructure.typeName]
			if !ok {
				ctx.addErr("cannot have a selection")
				return "null", true
			}
		}

		val := ctx.resolveSelection(query.selection, value, codeStructure, dept)
		return val, false
	case valueTypeData:
		if len(query.selection) > 0 {
			ctx.addErr("cannot have a selection")
			return "null", true
		}
		val, _ := valueToJson(value.Interface())
		if codeStructure.isID && codeStructure.dataValueType != reflect.String {
			// Graphql ID fields are always strings
			val = `"` + val + `"`
		}
		return val, false
	case valueTypePtr:
		if value.Kind() != reflect.Ptr || value.IsNil() {
			return "null", false
		}

		return ctx.resolveFieldDataValue(query, value.Elem(), codeStructure.innerContent, dept)
	case valueTypeEnum:
		enum := definedEnums[codeStructure.enumTypeName]

		key := enum.valueKey.MapIndex(value)
		if !key.IsValid() {
			return "null", false
		}
		return `"` + key.Interface().(string) + `"`, false
	case valueTypeTime:
		timeValue, ok := value.Interface().(time.Time)
		if !ok {
			return "null", false
		}
		timeString, _ := valueToJson(timeToString(timeValue))
		return timeString, false
	default:
		ctx.addErr("has invalid data type")
		return "null", true
	}
}

func valueToJson(in interface{}) (string, error) {
	switch v := in.(type) {
	case string:
		return fmt.Sprintf("%q", v), nil
	case bool:
		if v {
			return "true", nil
		} else {
			return "false", nil
		}
	case int:
		return fmt.Sprintf("%d", v), nil
	case int8:
		return fmt.Sprintf("%d", v), nil
	case int16:
		return fmt.Sprintf("%d", v), nil
	case int32: // == rune
		return fmt.Sprintf("%d", v), nil
	case int64:
		return fmt.Sprintf("%d", v), nil
	case uint:
		return fmt.Sprintf("%d", v), nil
	case uint8: // == byte
		return fmt.Sprintf("%d", v), nil
	case uint16:
		return fmt.Sprintf("%d", v), nil
	case uint32:
		return fmt.Sprintf("%d", v), nil
	case uint64:
		return fmt.Sprintf("%d", v), nil
	case uintptr:
		return fmt.Sprintf("%d", v), nil
	case float32:
		return floatToJson(32, float64(v)), nil
	case float64:
		return floatToJson(64, v), nil
	case *string:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *bool:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *int:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *int8:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *int16:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *int32: // = *rune
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *int64:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *uint:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *uint8: // = *byte
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *uint16:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *uint32:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *uint64:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *uintptr:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *float32:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	case *float64:
		if v == nil {
			return "null", nil
		}
		return valueToJson(*v)
	default:
		return "null", errors.New("invalid data type")
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

func (s *Schema) objToQlTypeName(item *obj) string {
	suffix := ""
	prefix := ""

	qlType := wrapQLTypeInNonNull(s.objToQLType(item))

	for {
		switch qlType.Kind {
		case typeKindList:
			suffix = "]" + suffix
			prefix += "["
		case typeKindNonNull:
			suffix = "!" + suffix
		default:
			if qlType.Name != nil {
				return prefix + *qlType.Name + suffix
			} else {
				return prefix + "Unknown" + suffix
			}
		}
		qlType = qlType.OfType
	}
}

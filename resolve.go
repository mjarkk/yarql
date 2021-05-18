package graphql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func (s *Schema) Resolve(query string, operatorTarget string) (string, []error) {
	s.m.Lock()
	defer s.m.Unlock()

	fragments, operatorsMap, errs := ParseQueryAndCheckNames(query)
	if len(errs) > 0 {
		return "{}", errs
	}

	ctx := &Ctx{
		fragments:  fragments,
		schema:     s,
		Values:     map[string]interface{}{},
		directvies: []Directives{},
		errors:     []error{},
	}

	switch len(operatorsMap) {
	case 0:
		return "{}", nil
	case 1:
		res := ""
		for _, operator := range operatorsMap {
			res = ctx.start(&operator)
		}
		return res, ctx.errors
	default:
		if operatorTarget == "" {
			return "{}", []error{errors.New("multiple operators without target")}
		}

		operator, ok := operatorsMap[operatorTarget]
		if ok {
			res := ctx.start(&operator)
			return res, ctx.errors
		} else {
			operatorsList := []string{}
			for k := range operatorsMap {
				operatorsList = append(operatorsList, k)
			}
			return "{}", []error{fmt.Errorf("%s is not a valid operator, available operators: %s", operatorTarget, strings.Join(operatorsList, ", "))}
		}
	}
}

func (ctx *Ctx) addErr(err string) {
	ctx.errors = append(ctx.errors, errors.New(err))
}

func (ctx *Ctx) addErrf(err string, args ...interface{}) {
	ctx.errors = append(ctx.errors, fmt.Errorf(err, args...))
}

func (ctx *Ctx) start(operator *Operator) string {
	// TODO add variables to exec ctx
	if operator.directives != nil && len(operator.directives) > 0 {
		ctx.directvies = append(ctx.directvies, operator.directives)
	}

	switch operator.operationType {
	case "query":
		return ctx.resolveSelection(operator.selection, ctx.schema.rootQueryValue, ctx.schema.rootQuery, 0)
	case "mutation":
		return ctx.resolveSelection(operator.selection, ctx.schema.rootQueryValue, ctx.schema.rootMethod, 0)
	case "subscription":
		// TODO
		ctx.addErr("subscription not suppored yet")
		return "{}"
	default:
		ctx.addErrf("%s cannot be used as operator", operator.operationType)
		return "{}"
	}
}

func (ctx *Ctx) resolveSelection(selectionSet SelectionSet, struct_ reflect.Value, structType *Obj, dept uint8) string {
	if dept >= ctx.schema.MaxDepth {
		return "null"
	}
	dept = dept + 1
	return "{" + ctx.resolveSelectionContent(selectionSet, struct_, structType, dept) + "}"
}

func (ctx *Ctx) resolveSelectionContent(selectionSet SelectionSet, struct_ reflect.Value, structType *Obj, dept uint8) string {
	res := ""
	writtenToRes := false
	for _, selection := range selectionSet {
		switch selection.selectionType {
		case "Field":
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

func (ctx *Ctx) resolveField(query *Field, struct_ reflect.Value, codeStructure *Obj, dept uint8) (fieldValue string, returnedOnError bool) {
	res := func(data string) string {
		name := query.name
		if len(query.alias) > 0 {
			name = query.alias
		}
		return fmt.Sprintf(`"%s":%s`, name, data)
	}

	structItem, ok := codeStructure.objContents[query.name]
	var value reflect.Value
	if !ok {
		ctx.addErrf("field %s does not exists on %s", query.name, codeStructure.typeName)
		return res("null"), true
	} else if structItem.valueType == valueTypeMethod && structItem.method.isTypeMethod {
		value = struct_.MethodByName(structItem.structFieldName)
	} else {
		value = struct_.FieldByName(structItem.structFieldName)
	}

	fieldValue, returnedOnError = ctx.resolveFieldDataValue(query, value, structItem, dept)
	return res(fieldValue), returnedOnError
}

func (ctx *Ctx) resolveFieldDataValue(query *Field, value reflect.Value, codeStructure *Obj, dept uint8) (fieldValue string, returnedOnError bool) {
	switch codeStructure.valueType {
	case valueTypeMethod:
		method := codeStructure.method

		inputs := []reflect.Value{}
		for _, in := range method.ins {
			if in.isCtx {
				inputs = append(inputs, reflect.ValueOf(ctx))
				continue
			}

			// TODO support function inputs
			ctx.addErrf("field %s does not yet have support for custom arguments", query.name)
			return "null", false
		}

		outs := value.Call(inputs)
		if method.errorOutNr != nil {
			errOut := outs[*method.errorOutNr]
			if !errOut.IsNil() {
				err, ok := errOut.Interface().(error)
				if !ok {
					ctx.addErrf("field %s returned a invalid kind of error", query.name)
					return "null", true
				} else if err != nil {
					ctx.addErr(err.Error())
				}
			}
		}

		return ctx.resolveFieldDataValue(query, outs[method.outNr], &method.outType, dept)
	case valueTypeArray:
		if (value.Kind() != reflect.Array && value.Kind() != reflect.Slice) || value.IsNil() {
			return "null", false
		}

		if codeStructure.innerContent == nil {
			ctx.addErrf("field %s does not have an internal type of an array", query.name)
			return "null", true
		}
		codeStructure = codeStructure.innerContent

		list := []string{}
		for i := 0; i < value.Len(); i++ {
			res, _ := ctx.resolveFieldDataValue(query, value.Index(i), codeStructure, dept)
			list = append(list, res)
		}
		return fmt.Sprintf("[%s]", strings.Join(list, ",")), false
	case valueTypeObj, valueTypeObjRef:
		if len(query.selection) == 0 {
			ctx.addErrf("field %s must have a selection", query.name)
			return "null", true
		}

		var ok bool
		if codeStructure.valueType == valueTypeObjRef {
			codeStructure, ok = ctx.schema.types[codeStructure.typeName]
			if !ok {
				ctx.addErrf("field %s cannot have a selection", query.name)
				return "null", true
			}
		}

		val := ctx.resolveSelection(query.selection, value, codeStructure, dept)
		return val, false
	case valueTypeData:
		if len(query.selection) > 0 {
			ctx.addErrf("field %s cannot have a selection", query.name)
			return "null", true
		}
		val, _ := valueToJson(value.Interface())
		return val, false
	case valueTypePtr:
		if value.Kind() != reflect.Ptr || value.IsNil() {
			return "null", false
		}

		return ctx.resolveFieldDataValue(query, value.Elem(), codeStructure.innerContent, dept)
	default:
		ctx.addErrf("field %s has invalid data type", query.name)
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

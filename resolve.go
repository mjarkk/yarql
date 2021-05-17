package graphql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// TODO check for query dept

func (s *Schema) Resolve(query string, operatorTarget string) (string, []error) {
	s.m.Lock()
	defer s.m.Unlock()

	fragments, operatorsMap, errs := ParseQueryAndCheckNames(query)
	if len(errs) > 0 {
		return "{}", errs
	}

	ctx := &ResolveCtx{
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

// ExecCtx contains all the request information and responses
type ResolveCtx struct {
	fragments  map[string]Operator    // Query fragments
	schema     *Schema                // The Go code schema (grahql schema)
	Values     map[string]interface{} // API User values, user can put all their shitty things in here like poems or tax papers
	directvies []Directives           // Directives stored in ctx
	errors     []error
}

func (ctx *ResolveCtx) addErr(err string) {
	ctx.errors = append(ctx.errors, errors.New(err))
}

func (ctx *ResolveCtx) addErrf(err string, args ...interface{}) {
	ctx.errors = append(ctx.errors, fmt.Errorf(err, args...))
}

func (ctx *ResolveCtx) start(operator *Operator) string {
	// TODO add variables to exec ctx
	if operator.directives != nil && len(operator.directives) > 0 {
		ctx.directvies = append(ctx.directvies, operator.directives)
	}

	switch operator.operationType {
	case "query":
		return ctx.resolveSelection(operator.selection, ctx.schema.rootQueryValue, ctx.schema.rootQuery)
	case "mutation":
		return ctx.resolveSelection(operator.selection, ctx.schema.rootQueryValue, ctx.schema.rootQuery)
	case "subscription":
		// TODO
		ctx.addErr("subscription not suppored yet")
		return "{}"
	default:
		ctx.addErrf("%s cannot be used as operator", operator.operationType)
		return "{}"
	}
}

func (ctx *ResolveCtx) resolveSelection(selectionSet SelectionSet, struct_ reflect.Value, structType *Obj) string {
	res := "{"
	writtenToRes := false
	for _, selection := range selectionSet {
		switch selection.selectionType {
		case "Field":
			value, hasError := ctx.resolveField(selection.field, struct_, structType)
			if !hasError {
				if writtenToRes {
					res += ","
				} else {
					writtenToRes = true
				}
				res += value
			}
		case "FragmentSpread":
			// TODO
			ctx.addErr("fragment spread currently unsupported")
			return "{}"
		case "InlineFragment":
			// TODO
			ctx.addErr("inline fragment currently unsupported")
			return "{}"
		}
	}
	return res + "}"
}

func (ctx *ResolveCtx) resolveField(query *Field, struct_ reflect.Value, codeStructure *Obj) (fieldValue string, returnedOnError bool) {
	structItem, ok := codeStructure.objContents[query.name]
	var value reflect.Value
	if !ok {
		method, ok := codeStructure.methods[query.name]
		if !ok {
			ctx.addErrf("field %s does not exists on %s", query.name, codeStructure.typeName)
			return "null", true
		}
		if method.isTypeMethod {
			value = struct_.FieldByName(method.methodName)
		} else {
			value = struct_.MethodByName(method.methodName)
		}
		// TODO
		ctx.addErrf("field %s uses function currently not supported", query.name)
		return "null", true
	} else {
		value = struct_.FieldByName(structItem.structFieldName)
	}
	if value.IsZero() {
		ctx.addErrf("field %s does not exists on %s", query.name, codeStructure.typeName)
		return "null", true
	}

	return ctx.resolveFieldDataValue(query, value, structItem)
}

func (ctx *ResolveCtx) resolveFieldDataValue(query *Field, value reflect.Value, codeStructure *Obj) (fieldValue string, returnedOnError bool) {
	okRes := func(data string) (string, bool) {
		name := query.name
		if len(query.alias) > 0 {
			name = query.alias
		}
		return fmt.Sprintf(`"%s":%s`, name, data), false
	}

	switch codeStructure.valueType {
	case valueTypeArray:
		// TODO yes you can actually have a selection
		// if len(query.selection) > 0 {
		// 	ctx.addErrf("field %s cannot have a selection", query.name)
		// 	return "", true
		// }

		if (value.Kind() != reflect.Array && value.Kind() != reflect.Slice) || value.IsNil() {
			return "null", false
		}

		if codeStructure.innerContent == nil {
			ctx.addErr("field %s does not have a internal array content type")
			return "null", true
		}
		codeStructure = codeStructure.innerContent

		for i := 0; i < value.Len(); i++ {

			value.Index(i)
		}
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

		return okRes(ctx.resolveSelection(query.selection, value, codeStructure))
	case valueTypeData:
		if len(query.selection) > 0 {
			ctx.addErrf("field %s cannot have a selection", query.name)
			return "null", true
		}
		val, _ := valueToJson(value.Interface())
		return okRes(val)
	case valueTypePtr:
		// TODO
	}

	ctx.addErrf("field %s has invalid data type", query.name)
	return "null", true
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

package graphql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func (s *Schema) Exec(query string, operatorTarget string) (string, []error) {
	s.m.Lock()
	defer s.m.Unlock()

	fragments, operatorsMap, errs := ParseQueryAndCheckNames(query)
	if len(errs) > 0 {
		return "{}", errs
	}

	ctx := &ExecCtx{
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
type ExecCtx struct {
	fragments  map[string]Operator    // Query fragments
	schema     *Schema                // The Go code schema (grahql schema)
	Values     map[string]interface{} // API User values, user can put all their shitty things in here like poems or tax papers
	directvies []Directives           // Directives stored in ctx
	errors     []error
}

func (ctx *ExecCtx) addErr(err string) {
	ctx.errors = append(ctx.errors, errors.New(err))
}

func (ctx *ExecCtx) addErrf(err string, args ...interface{}) {
	ctx.errors = append(ctx.errors, fmt.Errorf(err, args...))
}

func (ctx *ExecCtx) start(operator *Operator) string {
	// TODO add variables to exec ctx
	if operator.directives != nil && len(operator.directives) > 0 {
		ctx.directvies = append(ctx.directvies, operator.directives)
	}

	// TODO depending on the request type ctx.schema.rootQueryValue should be changed to root method value
	return ctx.parseSelection(operator.selection, ctx.schema.rootQueryValue, ctx.schema.rootQuery)
}

func (ctx *ExecCtx) parseSelection(selectionSet SelectionSet, struct_ reflect.Value, structType *Obj) string {
	res := "{"
	writtenToRes := false
	for _, selection := range selectionSet {
		switch selection.selectionType {
		case "Field":
			value, hasError := ctx.parseField(selection.field, struct_, structType)
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

func (ctx *ExecCtx) parseField(field *Field, struct_ reflect.Value, structType *Obj) (fieldValue string, returnedOnError bool) {
	okRes := func(data string) (string, bool) {
		name := field.name
		if len(field.alias) > 0 {
			name = field.alias
		}
		return fmt.Sprintf(`"%s":%s`, name, data), false
	}

	structItem, ok := structType.objContents[field.name]
	structFieldName := structItem.structFieldName
	var value reflect.Value
	if !ok {
		method, ok := structType.methods[field.name]
		if !ok {
			ctx.addErrf("field %s does not exists on %s", field.name, structType.typeName)
			return "", true
		}
		if method.isTypeMethod {
			value = struct_.FieldByName(structFieldName)
		} else {
			value = struct_.MethodByName(structFieldName)
		}
		// TODO
		ctx.addErrf("field %s uses function currently not supported", field.name)
		return "", true
	} else {
		value = struct_.FieldByName(structFieldName)
	}
	if value.IsZero() {
		ctx.addErrf("field %s does not exists on %s", field.name, structType.typeName)
		return "", true
	}

	switch structItem.valueType {
	case valueTypeArray:
		// TODO
	case valueTypeObj, valueTypeObjRef:
		if len(field.selection) == 0 {
			ctx.addErrf("field %s must have a selection", field.name)
			return "", true
		}

		if structItem.valueType == valueTypeObjRef {
			structItem, ok = ctx.schema.types[structItem.typeName]
			if !ok {
				ctx.addErrf("field %s cannot have a selection", field.name)
				return "", true
			}
		}

		return okRes(ctx.parseSelection(field.selection, value, structItem))
	case valueTypeData:
		switch structItem.dataValueType {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			if len(field.selection) > 0 {
				ctx.addErrf("field %s cannot have a selection", field.name)
				return "", true
			}

			return okRes(valueToJson(value.Interface()))
		case reflect.Struct, reflect.Map, reflect.UnsafePointer, reflect.Interface, reflect.Func, reflect.Chan, reflect.Uintptr, reflect.Complex64, reflect.Complex128:
			ctx.addErrf("field %s has an invalid data type", field.name)
			return "", true
		case reflect.Array, reflect.Slice:
			// TODO
		case reflect.Ptr:
			// TODO
		}
	case valueTypePtr:
		// TODO
	}

	ctx.addErrf("field %s has invalid data type", field.name)
	return "", true
}

func valueToJson(in interface{}) string {
	switch v := in.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case bool:
		if v {
			return "true"
		} else {
			return "false"
		}
	case int:
		return fmt.Sprintf("%d", v)
	case int8:
		return fmt.Sprintf("%d", v)
	case int16:
		return fmt.Sprintf("%d", v)
	case int32: // = rune
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case uint:
		return fmt.Sprintf("%d", v)
	case uint8: // = byte
		return fmt.Sprintf("%d", v)
	case uint16:
		return fmt.Sprintf("%d", v)
	case uint32:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case uintptr:
		return fmt.Sprintf("%d", v)
	case float32:
		return fmt.Sprintf("%e", v)
	case float64:
		return fmt.Sprintf("%e", v)
	case *string:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *bool:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *int:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *int8:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *int16:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *int32: // = *rune
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *int64:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *uint:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *uint8: // = *byte
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *uint16:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *uint32:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *uint64:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *uintptr:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *float32:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	case *float64:
		if v == nil {
			return "null"
		}
		return valueToJson(*v)
	default:
		return "null"
	}
}

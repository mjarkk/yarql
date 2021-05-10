package graphql

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"sync"
)

// Schema defines the graphql schema
type Schema struct {
	methods map[string]*Obj
	query   map[string]*Obj
	m       sync.Mutex
}

type valueType int

const (
	valueTypeFunction valueType = iota
	valueTypeArray
	valueTypeObj
	valueTypeData
	valueTypePtr
)

type Obj struct {
	valueType valueType

	// Value type == valueTypeObj
	objContents map[string]*Obj

	// Value is inside struct
	structFieldName string

	// Value type == valueTypeArray || type == valueTypePtr
	innerContent *Obj

	// Value type == valueTypeData
	dataValueType reflect.Kind

	m sync.Mutex
}

func ParseSchema(query map[string]interface{}, methods ...map[string]interface{}) (*Schema, error) {
	combinedMethods := map[string]interface{}{}
	for _, methodsObj := range methods {
		for k, v := range methodsObj {
			// TODO
			combinedMethods[k] = v
		}
	}

	res := Schema{
		methods: map[string]*Obj{},
		query:   map[string]*Obj{},
	}

	for key, value := range query {
		item, err := check(reflect.TypeOf(value))
		if err != nil {
			return nil, err
		}
		res.query[key] = item
	}

	return &res, nil
}

func check(t reflect.Type) (*Obj, error) {
	res := Obj{}

	switch t.Kind() {
	case reflect.Struct:
		res.valueType = valueTypeObj
		res.objContents = map[string]*Obj{}

		for i := 0; i < t.NumField(); i++ {
			err := func(field reflect.StructField) error {
				if field.Anonymous {
					return nil
				}

				val, ok := field.Tag.Lookup("gqIgnore")
				if ok && valueLooksTrue(val) {
					return nil
				}

				obj, err := check(field.Type)
				if err != nil {
					return err
				}
				obj.structFieldName = field.Name

				name := formatGoNameToQL(field.Name)
				newName, ok := field.Tag.Lookup("gqName")
				if ok {
					name = newName
				}

				res.objContents[name] = obj
				return nil
			}(t.Field(i))
			if err != nil {
				return nil, err
			}
		}
	case reflect.Array, reflect.Slice, reflect.Ptr:
		if t.Kind() == reflect.Ptr {
			res.valueType = valueTypePtr
		} else {
			res.valueType = valueTypeArray
		}

		obj, err := check(t.Elem())
		if err != nil {
			return nil, err
		}
		res.innerContent = obj
	case reflect.Func:
		// TODO
	case reflect.Map, reflect.Chan, reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Interface, reflect.UnsafePointer:
		return nil, errors.New("unsupported value type")
	default:
		res.valueType = valueTypeData
		res.dataValueType = t.Kind()
	}

	return &res, nil
}

func formatGoNameToQL(input string) string {
	if len(input) <= 1 {
		return strings.ToLower(input)
	}

	if input[1] == bytes.ToUpper([]byte{input[1]})[0] {
		// Don't change names like: INPUT to iNPUT
		return input
	}

	input = string(bytes.ToLower([]byte{input[0]})) + input[1:]

	return input
}

func valueLooksTrue(val string) bool {
	val = strings.ToLower(val)
	return val == "true" || val == "t" || val == "1"
}

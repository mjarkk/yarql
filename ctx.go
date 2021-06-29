package graphql

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"reflect"
	"strings"
	"time"

	"github.com/valyala/fastjson"
)

// Ctx contains all the request information and responses
type Ctx struct {
	// Private
	schema              *Schema         // The Go code schema (graphql schema)
	errors              []error         // Query errors
	operator            *operator       // Part of query to execute
	jsonVariablesString string          // Raw query variables
	jsonVariables       *fastjson.Value // Parsed query variables
	path                []byte
	context             context.Context
	getFormFile         func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
	extensions          map[string]interface{}
	tracingEnabled      bool
	tracing             *tracer

	// zero alloc values
	prefRecordingStartTime time.Time
	reflectValues          [256]reflect.Value
	currentReflectValueIdx uint8
	result                 []byte
	funcInputs             []reflect.Value

	// Public
	Values map[string]interface{} // API User values, user can put all their shitty things in here like poems or tax papers
}

//
// External
//

func (ctx *Ctx) GetExtension(key string) (value interface{}, ok bool) {
	if ctx.extensions == nil {
		return nil, false
	}
	value, ok = ctx.extensions[key]
	return
}

func (ctx *Ctx) SetExtension(key string, value interface{}) {
	if ctx.extensions == nil {
		ctx.extensions = map[string]interface{}{}
	}
	ctx.extensions[key] = value
}

// Returns the request's context
func (ctx *Ctx) Context() context.Context {
	return ctx.context
}

// Path to the current method encoded as json
func (ctx *Ctx) Path() string {
	if len(ctx.path) == 0 {
		return `[]`
	}
	return `[` + string(ctx.path[1:]) + `]`
}

// HasErrors checks if the query has errors till this current point of execution
// Note that due to maps beeing read randomly this might be diffrent when executing a equal query
func (ctx *Ctx) HasErrors() bool {
	return len(ctx.errors) > 0
}

// Errors returns the query errors til this point
func (ctx *Ctx) Errors() []error {
	if ctx.errors == nil {
		return []error{}
	}
	return ctx.errors
}

// AddError add an error to the query
func (ctx *Ctx) AddError(err error) {
	if len(ctx.path) == 0 {
		ctx.errors = append(ctx.errors, err)
	} else {
		copiedPath := make([]byte, len(ctx.path)-1)
		copy(copiedPath, ctx.path[1:])

		ctx.errors = append(ctx.errors, ErrorWPath{
			err:  err,
			path: copiedPath,
		})
	}
}

//
// Internal
//

func (ctx *Ctx) writeString(val string) {
	ctx.result = append(ctx.result, []byte(val)...)
}

func (ctx *Ctx) write(val []byte) {
	ctx.result = append(ctx.result, val...)
}

func (ctx *Ctx) writeByte(val byte) {
	ctx.result = append(ctx.result, val)
}

func (ctx *Ctx) value() reflect.Value {
	return ctx.reflectValues[ctx.currentReflectValueIdx]
}

func (ctx *Ctx) addErr(err string) {
	if len(ctx.path) == 0 {
		ctx.errors = append(ctx.errors, errors.New(err))
	} else {
		copiedPath := make([]byte, len(ctx.path)-1)
		copy(copiedPath, ctx.path[1:])

		ctx.errors = append(ctx.errors, ErrorWPath{
			err:  errors.New(err),
			path: copiedPath,
		})
	}
}

func (ctx *Ctx) addErrf(err string, args ...interface{}) {
	if len(ctx.path) == 0 {
		ctx.errors = append(ctx.errors, fmt.Errorf(err, args...))
	} else {
		copiedPath := make([]byte, len(ctx.path)-1)
		copy(copiedPath, ctx.path[1:])

		ctx.errors = append(ctx.errors, ErrorWPath{
			err:  fmt.Errorf(err, args...),
			path: copiedPath,
		})
	}
}

// getVariable tries to resolve a variable and places it inside the value argument
// Variable must be defined inside the operator
func (ctx *Ctx) getVariable(name string, value *value) error {
	definition, ok := ctx.operator.variableDefinitions[name]
	if !ok {
		return errors.New("variable not defined in " + ctx.operator.operationType)
	}

	jsonVariables, err := ctx.getJSONVariables()
	if err != nil {
		return err
	}
	jsonVariable := jsonVariables.Get(name)

	if jsonVariable != nil {
		// Resolve from json
		return ctx.resolveVariableFromJson(jsonVariable, &definition.varType, value)
	}

	if definition.defaultValue != nil {
		// Resolve from default value
		return ctx.resolveVariableFromDefault(*definition.defaultValue, &definition.varType, value)
	}

	// Return null value if no value is provided, depending on the input will cause an error but thats expected
	value.isNull = true
	return nil
}

func (ctx *Ctx) resolveVariableFromJson(jsonValue *fastjson.Value, expectedValueType *typeReference, val *value) error {
	if expectedValueType.list {
		arrContents, err := jsonValue.Array()
		if err != nil {
			return err
		}
		newArray := []value{}
		for _, item := range arrContents {
			if item == nil {
				continue
			}

			itemValue := value{}
			err = ctx.resolveVariableFromJson(item, expectedValueType.listType, &itemValue)
			if err != nil {
				return err
			}
			newArray = append(newArray, itemValue)
		}
		val.valType = reflect.Array
		val.listValue = newArray
		return nil
	}

	// TODO support ID values

	val.qlTypeName = &expectedValueType.name
	var err error
	switch expectedValueType.name {
	case "Boolean":
		val.valType = reflect.Bool
		val.booleanValue, err = jsonValue.Bool()
		return err
	case "Int":
		val.valType = reflect.Int
		val.intValue, err = jsonValue.Int()
		return err
	case "Float":
		val.valType = reflect.Float64
		val.floatValue, err = jsonValue.Float64()
		return err
	case "String":
		val.valType = reflect.String
		strVal, err := jsonValue.StringBytes()
		if err != nil {
			return err
		}
		val.stringValue = string(strVal)
		return nil
	}

	_, ok := ctx.schema.inTypes[expectedValueType.name]
	if ok {
		jsonObject, err := jsonValue.Object()
		if err != nil {
			return errors.New("exected default value to be of kind object")
		}

		objectContent := arguments{}
		jsonObject.Visit(func(key []byte, v *fastjson.Value) {
			if v != nil {
				keyStr := string(key)
				objectContent[keyStr] = jsonValueToValue(v)
			}
		})

		val.valType = reflect.Map
		val.objectValue = objectContent
		return nil
	}

	// FIXME this is slow
	for _, enum := range definedEnums {
		if enum.typeName == expectedValueType.name {
			val.isEnum = true
			strVal, err := jsonValue.StringBytes()
			if err != nil {
				return errors.New("expected enum value as string but got " + jsonValue.Type().String())
			}
			val.enumValue = string(strVal)
			return nil
		}
	}

	return errors.New("Unknown input type " + expectedValueType.name)
}

func (ctx *Ctx) resolveVariableFromDefault(defaultValue value, expectedValueType *typeReference, val *value) error {
	if expectedValueType.list {
		if defaultValue.valType != reflect.Array {
			return errors.New("exected list")
		}
		newArray := []value{}
		for _, listItem := range defaultValue.listValue {
			itemValue := value{}
			err := ctx.resolveVariableFromDefault(listItem, expectedValueType.listType, &itemValue)
			if err != nil {
				return err
			}
			newArray = append(newArray, itemValue)
		}
		val.valType = reflect.Array
		val.listValue = newArray
		return nil
	}

	// TODO support ID values

	val.qlTypeName = &expectedValueType.name
	switch expectedValueType.name {
	case "Boolean":
		return val.setToValueOfAndExpect(defaultValue, reflect.Bool)
	case "Int":
		return val.setToValueOfAndExpect(defaultValue, reflect.Int)
	case "Float":
		return val.setToValueOfAndExpect(defaultValue, reflect.Float64)
	case "String":
		return val.setToValueOfAndExpect(defaultValue, reflect.String)
	}

	_, ok := ctx.schema.inTypes[expectedValueType.name]
	if ok {
		if defaultValue.valType != reflect.Map {
			return errors.New("exected default value to be of kind object")
		}
		val.valType = reflect.Map
		val.objectValue = defaultValue.objectValue
		return nil
	}

	// FIXME this is slow
	for _, enum := range definedEnums {
		if enum.typeName == expectedValueType.name {
			if !defaultValue.isEnum {
				return errors.New("exected default value to be of kind enum")
			}
			val.isEnum = true
			val.enumValue = defaultValue.enumValue
			return nil
		}
	}

	return errors.New("Unknown input type " + expectedValueType.name)
}

func (ctx *Ctx) getJSONVariables() (*fastjson.Value, error) {
	if ctx.jsonVariables != nil {
		return ctx.jsonVariables, nil
	}

	ctx.jsonVariablesString = strings.TrimSpace(ctx.jsonVariablesString)
	if ctx.jsonVariablesString == "" {
		ctx.jsonVariablesString = "{}"
	}
	var p fastjson.Parser
	v, err := p.Parse(ctx.jsonVariablesString)
	if err != nil {
		return nil, err
	}
	if v.Type() != fastjson.TypeObject {
		return nil, errors.New("variables provided must be of type object")
	}

	ctx.jsonVariables = v
	return ctx.jsonVariables, nil
}

func jsonValueToValue(jsonValue *fastjson.Value) value {
	switch jsonValue.Type() {
	case fastjson.TypeNull:
		return makeNullValue()
	case fastjson.TypeObject:
		objectContent := arguments{}
		jsonValue.GetObject().Visit(func(key []byte, v *fastjson.Value) {
			keyStr := string(key)
			objectContent[keyStr] = jsonValueToValue(v)
		})
		return makeStructValue(objectContent)
	case fastjson.TypeArray:
		list := []value{}
		for _, item := range jsonValue.GetArray() {
			if item == nil {
				continue
			}
			list = append(list, jsonValueToValue(item))
		}
		return makeArrayValue(list)
	case fastjson.TypeString:
		return makeStringValue(string(jsonValue.GetStringBytes()))
	case fastjson.TypeNumber:
		intVal, err := jsonValue.Int()
		if err != nil {
			return makeFloatValue(jsonValue.GetFloat64())
		}
		return makeIntValue(intVal)
	case fastjson.TypeTrue, fastjson.TypeFalse:
		return makeBooleanValue(jsonValue.GetBool())
	}
	return makeNullValue()
}

type ErrorWPath struct {
	err  error
	path []byte // a json representation of the path without the [] around it
}

func (e ErrorWPath) Error() string {
	return e.err.Error()
}

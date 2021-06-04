package graphql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/valyala/fastjson"
)

// Ctx contains all the request information and responses
type Ctx struct {
	fragments           map[string]Operator    // Query fragments
	schema              *Schema                // The Go code schema (graphql schema)
	Values              map[string]interface{} // API User values, user can put all their shitty things in here like poems or tax papers
	directvies          []Directives           // Directives stored in ctx
	errors              []error                // Query errors
	operator            *Operator              // Part of query to execute
	jsonVariablesString string                 // Raw query variables
	jsonVariables       *fastjson.Value        // Parsed query variables
}

//
// External
//

// HasErrors checks if the query has errors till this current point of execution
// Note that due to maps beeing read randomly this might be diffrent when executing a equal query
func (ctx *Ctx) HasErrors() bool {
	return len(ctx.errors) > 0
}

// Errors returns the query errors til this point
func (ctx *Ctx) Errors() []error {
	return ctx.errors
}

// AddError add an error to the query
func (ctx *Ctx) AddError(err error) {
	ctx.errors = append(ctx.errors, err)
}

//
// Internal
//

func (ctx *Ctx) addErr(err string) {
	ctx.errors = append(ctx.errors, errors.New(err))
}

func (ctx *Ctx) addErrf(err string, args ...interface{}) {
	ctx.errors = append(ctx.errors, fmt.Errorf(err, args...))
}

// getVariable tries to resolve a variable and places it inside the value argument
// Variable must be defined inside the operator
func (ctx *Ctx) getVariable(name string, value *Value) error {
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

func (ctx *Ctx) resolveVariableFromJson(jsonValue *fastjson.Value, expectedValueType *TypeReference, value *Value) error {
	if expectedValueType.list {
		arrContents, err := jsonValue.Array()
		if err != nil {
			return err
		}
		newArray := []Value{}
		for _, item := range arrContents {
			if item == nil {
				continue
			}

			itemValue := Value{}
			err = ctx.resolveVariableFromJson(item, expectedValueType.listType, &itemValue)
			if err != nil {
				return err
			}
			newArray = append(newArray, itemValue)
		}
		value.valType = reflect.Array
		value.listValue = newArray
		return nil
	}

	// TODO support struct and ID values

	var err error
	switch expectedValueType.name {
	case "Boolean":
		value.valType = reflect.Bool
		value.booleanValue, err = jsonValue.Bool()
		return err
	case "Int":
		value.valType = reflect.Int
		value.intValue, err = jsonValue.Int()
		return err
	case "Float":
		value.valType = reflect.Float64
		value.floatValue, err = jsonValue.Float64()
		return err
	case "String":
		value.valType = reflect.String
		val, err := jsonValue.StringBytes()
		if err != nil {
			return err
		}
		value.stringValue = string(val)
		return nil
	}

	_, ok := definedEnums[expectedValueType.name]
	if ok {
		value.isEnum = true
		val, err := jsonValue.StringBytes()
		if err != nil {
			return errors.New("expected enum value as string but got " + jsonValue.Type().String())
		}
		value.enumValue = string(val)
		return nil
	}

	return errors.New("Unknown input type " + expectedValueType.name)
}

func (ctx *Ctx) resolveVariableFromDefault(defaultValue Value, expectedValueType *TypeReference, value *Value) error {
	if expectedValueType.list {
		if defaultValue.valType != reflect.Array {
			return errors.New("exected list")
		}
		newArray := []Value{}
		for _, listItem := range defaultValue.listValue {
			itemValue := Value{}
			err := ctx.resolveVariableFromDefault(listItem, expectedValueType.listType, &itemValue)
			if err != nil {
				return err
			}
			newArray = append(newArray, itemValue)
		}
		value.valType = reflect.Array
		value.listValue = newArray
		return nil
	}

	// TODO support struct and ID values

	switch expectedValueType.name {
	case "Boolean":
		return value.SetToValueOfAndExpect(defaultValue, reflect.Bool)
	case "Int":
		return value.SetToValueOfAndExpect(defaultValue, reflect.Int)
	case "Float":
		return value.SetToValueOfAndExpect(defaultValue, reflect.Float64)
	case "String":
		return value.SetToValueOfAndExpect(defaultValue, reflect.String)
	}

	_, ok := definedEnums[expectedValueType.name]
	if ok {
		if !defaultValue.isEnum {
			return errors.New("exected default value to be of kind enum")
		}
		value.isEnum = true
		value.enumValue = defaultValue.enumValue
		return nil
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

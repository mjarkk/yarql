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

	if definition.varType.list {

	}

	typeName := definition.varType.name
	qlType := ctx.schema.getTypeByName(typeName, true, false)
	if qlType == nil {
		return fmt.Errorf("unknown variable %s type %s", name, typeName)
	}

	jsonVariables, err := ctx.getJSONVariables()
	if err != nil {
		return err
	}
	jsonVariable := jsonVariables.Get(name)

	defaultValue := definition.defaultValue
	if qlType.Kind == TypeKindScalar {
		switch *qlType.Name {
		case "Boolean":
			value.valType = reflect.Bool
			if jsonVariable != nil {
				val, err := jsonVariable.Bool()
				if err != nil {
					return err
				}
				value.booleanValue = val
			} else if defaultValue != nil {
				if defaultValue.valType != reflect.Bool {
					return fmt.Errorf("default value of %s doesn't match it's type", name)
				}
				value.booleanValue = defaultValue.booleanValue
			}
			return nil
		case "Int":
			value.valType = reflect.Int
			if jsonVariable != nil {
				val, err := jsonVariable.Int()
				if err != nil {
					return err
				}
				value.intValue = val
			} else if defaultValue != nil {
				if defaultValue.valType != reflect.Int {
					return fmt.Errorf("default value of %s doesn't match it's type", name)
				}
				value.intValue = defaultValue.intValue
			}
		case "Float":
			value.valType = reflect.Float64
			if jsonVariable != nil {
				val, err := jsonVariable.Float64()
				if err != nil {
					return err
				}
				value.floatValue = val
			} else if defaultValue != nil {
				if defaultValue.valType != reflect.Float64 {
					return fmt.Errorf("default value of %s doesn't match it's type", name)
				}
				value.floatValue = defaultValue.floatValue
			}
		case "String":
			value.valType = reflect.String
			if jsonVariable != nil {
				val, err := jsonVariable.StringBytes()
				if err != nil {
					return err
				}
				value.stringValue = string(val)
			} else if defaultValue != nil {
				if defaultValue.valType != reflect.String {
					return fmt.Errorf("default value of %s doesn't match it's type", name)
				}
				value.stringValue = defaultValue.stringValue
			}
		default:
			return errors.New("Unexpected input type " + *qlType.Name)
		}
		return nil
	}

	// TODO: Support more kinds
	return fmt.Errorf("variable %s of kind %s is currently unsupported", name, qlType.Kind.String())
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

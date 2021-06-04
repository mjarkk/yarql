package graphql

import (
	"errors"
	"reflect"
)

type value struct {
	// Check these before valType
	isVar  bool
	isNull bool
	isEnum bool

	// depending on this field the below is filled in
	// Supported: Int, Float64, String, Bool, Array, Map
	// Maybe we should rename Map to Struct everywhere
	valType reflect.Kind

	variable     string
	intValue     int
	floatValue   float64
	stringValue  string
	booleanValue bool
	enumValue    string
	listValue    []value
	objectValue  arguments

	// Set this value if the value might be used on multiple places and the graphql typename is known
	// When using this struct to set data and this field is defined you should check it
	qlTypeName *string
}

func (v *value) setToValueOfAndExpect(other value, expect reflect.Kind) error {
	if other.valType != expect {
		return errors.New("Value expected to be of type " + expect.String())
	}
	v.setToValueOf(other)
	return nil
}

func (v *value) setToValueOf(other value) {
	v.valType = other.valType
	switch other.valType {
	case reflect.String:
		v.stringValue = other.stringValue
	case reflect.Int:
		v.intValue = other.intValue
	case reflect.Float64:
		v.floatValue = other.floatValue
	case reflect.Bool:
		v.booleanValue = other.booleanValue
	case reflect.Array:
		v.listValue = other.listValue
	case reflect.Map:
		v.objectValue = other.objectValue
	}
}

func makeStringValue(val string) value {
	return value{
		valType:     reflect.String,
		stringValue: val,
	}
}

func makeBooleanValue(val bool) value {
	return value{
		valType:      reflect.Bool,
		booleanValue: val,
	}
}

func makeIntValue(val int) value {
	return value{
		valType:  reflect.Int,
		intValue: val,
	}
}

func makeFloatValue(val float64) value {
	return value{
		valType:    reflect.Float64,
		floatValue: val,
	}
}

func makeEnumValue(val string) value {
	return value{
		isEnum:    true,
		enumValue: val,
	}
}

func makeNullValue() value {
	return value{
		isNull: true,
	}
}

func makeArrayValue(list []value) value {
	if list == nil {
		list = []value{}
	}
	return value{
		valType:   reflect.Array,
		listValue: list,
	}
}

func makeStructValue(keyValues arguments) value {
	if keyValues == nil {
		keyValues = arguments{}
	}
	return value{
		valType:     reflect.Map,
		objectValue: keyValues,
	}
}

func makeVariableValue(varName string) value {
	return value{
		variable: varName,
		isVar:    true,
	}
}

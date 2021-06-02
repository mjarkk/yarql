package graphql

import "reflect"

type Value struct {
	// Check these before valType
	isVar  bool
	isNull bool
	isEnum bool

	// depending on this field the below is filled in
	// Supported: Int, Float64, String, Bool, Array, Map
	valType reflect.Kind

	variable     string
	intValue     int
	floatValue   float64
	stringValue  string
	booleanValue bool
	enumValue    string
	listValue    []Value
	objectValue  Arguments
}

func makeStringValue(val string) Value {
	return Value{
		valType:     reflect.String,
		stringValue: val,
	}
}

func makeBooleanValue(val bool) Value {
	return Value{
		valType:      reflect.Bool,
		booleanValue: val,
	}
}

func makeIntValue(val int) Value {
	return Value{
		valType:  reflect.Int,
		intValue: val,
	}
}

func makeFloatValue(val float64) Value {
	return Value{
		valType:    reflect.Float64,
		floatValue: val,
	}
}

func makeEnumValue(val string) Value {
	return Value{
		isEnum:    true,
		enumValue: val,
	}
}

func makeNullValue() Value {
	return Value{
		isNull: true,
	}
}

func makeArrayValue(list []Value) Value {
	if list == nil {
		list = []Value{}
	}
	return Value{
		valType:   reflect.Array,
		listValue: list,
	}
}

func makeStructValue(keyValues Arguments) Value {
	if keyValues == nil {
		keyValues = Arguments{}
	}
	return Value{
		valType:     reflect.Map,
		objectValue: keyValues,
	}
}

func makeVariableValue(varName string) Value {
	return Value{
		variable: varName,
		isVar:    true,
	}
}

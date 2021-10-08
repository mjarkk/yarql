package graphql

import (
	"reflect"
)

// implementationMap is a map of interface names and the types that implement them
var implementationMap = map[string][]reflect.Type{}

// structImplementsMap is list of all structs and their interfaces that they implement
var structImplementsMap = map[string][]reflect.Type{}

// Implements registers a new type that implementation an interface
// The interfaceValue should be a pointer to the interface type like: (*InterfaceType)(nil)
// The typeValue should be a empty struct that implements the interfaceValue
//
// Example:
//   var _ = Implements((*InterfaceType)(nil), StructThatImplements{})
func Implements(interfaceValue interface{}, typeValue interface{}) bool {
	if interfaceValue == nil {
		panic("interfaceValue cannot be nil")
	}
	interfaceType := reflect.TypeOf(interfaceValue)
	if interfaceType.Kind() != reflect.Ptr {
		panic("interfaceValue should be a pointer to a interface")
	}
	interfaceType = interfaceType.Elem()
	if interfaceType.Kind() != reflect.Interface {
		panic("interfaceValue should be a pointer to a interface")
	}

	interfaceName := interfaceType.Name()
	interfacePath := interfaceType.PkgPath()
	if interfaceName == "" || interfacePath == "" {
		panic("interfaceValue should be a pointer to a named interface, not a inline interface")
	}

	if typeValue == nil {
		panic("typeValue cannot be nil")
	}
	typeType := reflect.TypeOf(typeValue)
	if typeType.Kind() != reflect.Struct {
		panic("typeValue must be a struct")
	}

	typeName := typeType.Name()
	typePath := typeType.PkgPath()
	if typeName == "" || typePath == "" {
		panic("typeName should is not allowed to be a inline struct")
	}

	if !typeType.Implements(interfaceType) {
		panic(typePath + "." + typePath + " does not implement " + interfacePath + "." + interfaceName)
	}

	typesThatImplementInterf, ok := implementationMap[interfaceName]
	if !ok {
		typesThatImplementInterf = []reflect.Type{}
	} else {
		for _, type_ := range typesThatImplementInterf {
			if type_.Name() == typeName && type_.PkgPath() == typePath {
				// already registered
				return true
			}
		}
	}
	typesThatImplementInterf = append(typesThatImplementInterf, typeType)
	implementationMap[interfaceName] = typesThatImplementInterf

	structImplementsMap[typeName] = append(structImplementsMap[typeName], interfaceType)

	return true
}

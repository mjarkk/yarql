package graphql

import (
	"fmt"
	"reflect"
	"strings"
)

type enum struct {
	contentType reflect.Type
	typeName    string
	keyValue    map[string]reflect.Value
	valueKey    reflect.Value
}

func getEnum(t reflect.Type) *enum {
	if len(t.PkgPath()) == 0 || len(t.Name()) == 0 || !validEnumType(t) {
		return nil
	}

	enum, ok := definedEnums[t.Name()]
	if !ok {
		return nil
	}

	return &enum
}

func validEnumType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// All int kinds are allowed
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// All uint kinds are allowed
		return true
	case reflect.String:
		// Strings are allowed
		return true
	default:
		return false
	}
}

var definedEnums = map[string]enum{}

func RegisterEnum(map_ interface{}) bool {
	enum := registerEnumCheck(map_)
	if enum == nil {
		return false
	}

	definedEnums[enum.typeName] = *enum
	return true
}

func registerEnumCheck(map_ interface{}) *enum {
	mapReflection := reflect.ValueOf(map_)
	mapType := mapReflection.Type()

	invalidTypeMsg := fmt.Sprintf("RegisterEnum input must be of type map[string]CustomType(int..|uint..|string) as input, %+v given", map_)

	if mapType.Kind() != reflect.Map {
		// Tye input type must be a map
		panic(invalidTypeMsg)
	}
	if mapType.Key().Kind() != reflect.String {
		// The map key must be a string
		panic(invalidTypeMsg)
	}
	contentType := mapType.Elem()
	if !validEnumType(contentType) {
		panic(invalidTypeMsg)
	}

	if contentType.PkgPath() == "" || contentType.Name() == "" {
		panic("RegisterEnum input map value must have a global custom type value (type Animals string) or (type Rules uint64)")
	}

	inputLen := mapReflection.Len()
	if inputLen == 0 {
		// No point in registering enums with 0 items
		return nil
	}

	res := map[string]reflect.Value{}
	valueKeyMap := reflect.MakeMapWithSize(reflect.MapOf(contentType, reflect.TypeOf("")), inputLen)

	iter := mapReflection.MapRange()
	for iter.Next() {
		k := iter.Key()
		keyStr := k.Interface().(string)
		if keyStr == "" {
			panic("RegisterEnum input map cannot contain empty keys")
		}

		keyByteArr := []byte(keyStr)

		letters := "abcdefghijklmnopqrstuvwxyz"
		numbers := "0123456789"
		special := "_"

		valid := false
		for _, letter := range []byte(letters + strings.ToUpper(letters)) {
			if keyByteArr[0] == letter {
				valid = true
				break
			}
		}
		if !valid {
			panic(fmt.Sprintf("RegisterEnum map key must start with an alphabetic character (lower or upper), key given: %s", keyStr))
		}
		for _, keyLetter := range keyByteArr[1:] {
			valid = false
			for _, letter := range []byte(letters + strings.ToUpper(letters) + numbers + special) {
				if keyLetter == letter {
					valid = true
					break
				}
			}
			if !valid {
				panic(fmt.Sprintf("RegisterEnum map key must match [a-zA-Z][a-zA-Z0-9_]*, key given: %s", keyStr))
			}
		}

		v := iter.Value()
		if valueKeyMap.MapIndex(v).IsValid() {
			panic(fmt.Sprintf("RegisterEnum input map cannot have duplicated values, value: %+v", v.Interface()))
		}
		valueKeyMap.SetMapIndex(v, k)
		res[keyStr] = v
	}

	return &enum{
		contentType: contentType,
		keyValue:    res,
		valueKey:    valueKeyMap,
		typeName:    contentType.Name(),
	}
}

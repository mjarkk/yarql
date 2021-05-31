package graphql

import (
	"fmt"
	"reflect"
	"strings"
)

type enum struct {
	contentType reflect.Type
	options     map[string]interface{}
}

func getEnumKey(t reflect.Type) string {
	return t.PkgPath() + " " + t.Name()
}

var definedEnums = map[string]enum{}

func RegisterEnum(map_ interface{}) {
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
	switch contentType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// All int kinds are allowed
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// All uint kinds are allowed
	case reflect.String:
		// Strings are allowed
	default:
		panic(invalidTypeMsg)
	}

	if contentType.PkgPath() == "" || contentType.Name() == "" {
		panic("RegisterEnum input map value must have a global custom type value (type Animals string) or (type Rules uint64)")
	}

	inputLen := mapReflection.Len()
	if inputLen == 0 {
		// No point in registering enums with 0 items
		return
	}

	res := map[string]interface{}{}
	valuesMap := reflect.MakeMapWithSize(reflect.MapOf(contentType, reflect.TypeOf(true)), inputLen)
	trueReflectValue := reflect.ValueOf(true)

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
		vInterface := v.Interface()
		if valuesMap.MapIndex(v).IsValid() {
			panic(fmt.Sprintf("RegisterEnum input map cannot have duplicated values, value: %+v", vInterface))
		}
		valuesMap.SetMapIndex(v, trueReflectValue)
		res[keyStr] = vInterface
	}

	definedEnums[getEnumKey(contentType)] = enum{
		contentType: contentType,
		options:     res,
	}
}

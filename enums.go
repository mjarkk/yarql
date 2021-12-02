package graphql

import (
	"errors"
	"fmt"
	"reflect"
	"sort"

	h "github.com/mjarkk/go-graphql/helpers"
)

type enum struct {
	contentType reflect.Type
	contentKind reflect.Kind
	typeName    string
	entries     []enumEntry
	qlType      qlType
}

type enumEntry struct {
	keyBytes []byte
	key      string
	value    reflect.Value
}

func (s *Schema) getEnum(t reflect.Type) (int, *enum) {
	if len(t.PkgPath()) == 0 || len(t.Name()) == 0 || !validEnumType(t) {
		return -1, nil
	}

	for i, enum := range s.definedEnums {
		if enum.typeName == t.Name() {
			return i, &enum
		}
	}

	return -1, nil
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

// RegisterEnum registers a new enum type
func (s *Schema) RegisterEnum(enumMap interface{}) (added bool, err error) {
	if s.parsed {
		return false, errors.New("(*graphql.Schema).RegisterEnum() cannot be ran after (*graphql.Schema).Parse()")
	}

	enum, err := registerEnumCheck(enumMap)
	if enum == nil || err != nil {
		return false, err
	}

	s.definedEnums = append(s.definedEnums, *enum)
	return true, nil
}

func registerEnumCheck(enumMap interface{}) (*enum, error) {
	mapReflection := reflect.ValueOf(enumMap)
	invalidTypeMsg := fmt.Errorf("RegisterEnum input must be of type map[string]CustomType(int..|uint..|string) as input, %+v given", enumMap)

	if enumMap == nil || mapReflection.IsZero() || mapReflection.IsNil() {
		return nil, invalidTypeMsg
	}

	mapType := mapReflection.Type()

	if mapType.Kind() != reflect.Map {
		// Tye input type must be a map
		return nil, invalidTypeMsg
	}
	if mapType.Key().Kind() != reflect.String {
		// The map key must be a string
		return nil, invalidTypeMsg
	}
	contentType := mapType.Elem()
	if !validEnumType(contentType) {
		return nil, invalidTypeMsg
	}

	if contentType.PkgPath() == "" || contentType.Name() == "" {
		return nil, errors.New("RegisterEnum input map value must have a global custom type value (type Animals string) or (type Rules uint64)")
	}

	inputLen := mapReflection.Len()
	if inputLen == 0 {
		// No point in registering enums with 0 items
		return nil, nil
	}

	entries := make([]enumEntry, inputLen)
	qlTypeEnumValues := make([]qlEnumValue, inputLen)

	iter := mapReflection.MapRange()
	i := 0
	for iter.Next() {
		k := iter.Key()
		keyStr := k.Interface().(string)
		if keyStr == "" {
			return nil, errors.New("RegisterEnum input map cannot contain empty keys")
		}

		err := validGraphQlName([]byte(keyStr))
		if err != nil {
			return nil, errors.New(`RegisterEnum map key must start with an alphabetic character (lower or upper) followed by the same or a "_", key given: ` + keyStr)
		}

		entries[i] = enumEntry{
			keyBytes: []byte(keyStr),
			key:      keyStr,
			value:    iter.Value(),
		}
		qlTypeEnumValues[i] = qlEnumValue{
			Name:              keyStr,
			Description:       h.PtrToEmptyStr,
			IsDeprecated:      false,
			DeprecationReason: nil,
		}
		i++
	}
	sort.Slice(qlTypeEnumValues, func(a int, b int) bool { return qlTypeEnumValues[a].Name < qlTypeEnumValues[b].Name })

	name := contentType.Name()
	qlType := qlType{
		Kind:        typeKindEnum,
		Name:        &name,
		Description: h.PtrToEmptyStr,
		EnumValues:  func(args isDeprecatedArgs) []qlEnumValue { return qlTypeEnumValues },
	}

	return &enum{
		contentType: contentType,
		contentKind: contentType.Kind(),
		entries:     entries,
		typeName:    name,
		qlType:      qlType,
	}, nil
}

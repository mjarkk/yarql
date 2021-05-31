package graphql

import (
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestRegisterEnum(t *testing.T) {
	type TestEnum string

	firstLen := len(definedEnums)

	RegisterEnum(map[string]TestEnum{
		"A": "B",
	})

	Less(t, firstLen, len(definedEnums))
}

func TestEmptyEnumShouldNotBeRegistered(t *testing.T) {
	type TestEnum string

	firstLen := len(definedEnums)

	RegisterEnum(map[string]TestEnum{})

	Equal(t, firstLen, len(definedEnums), "defined enums should not grow in size when adding a empty enum")
}

func TestRegisterEnumFails(t *testing.T) {
	type TestEnum string

	Panics(t, func() {
		RegisterEnum(0)
	}, "Cannot generate an enum of non map types")

	Panics(t, func() {
		RegisterEnum(nil)
	}, "Cannot generate an enum of non map types 2")

	Panics(t, func() {
		RegisterEnum(map[int]TestEnum{
			1: "a",
		})
	}, "Enum must have a string key type")

	Panics(t, func() {
		RegisterEnum(map[string]struct{}{
			"a": {},
		})
	}, "Enum value cannot be complex")

	Panics(t, func() {
		RegisterEnum(map[string]string{
			"foo": "bar",
		})
	}, "Enum value must be a custom type")

	Panics(t, func() {
		RegisterEnum(map[string]TestEnum{
			"": "",
		})
	}, "Enum keys cannot be empty")

	Panics(t, func() {
		RegisterEnum(map[string]TestEnum{
			"Foo": "Baz",
			"Bar": "Baz",
		})
	}, "Enum cannot have duplicated values")

	Panics(t, func() {
		RegisterEnum(map[string]TestEnum{
			"1": "",
		})
	}, "Enum cannot have an invalid graphql name, where first letter is number")

	Panics(t, func() {
		RegisterEnum(map[string]TestEnum{
			"_": "",
		})
	}, "Enum cannot have an invalid graphql name, where first letter is underscore")

	Panics(t, func() {
		RegisterEnum(map[string]TestEnum{
			"A!!!!": "",
		})
	}, "Enum cannot have an invalid graphql name, where remainder of name is invalid")
}

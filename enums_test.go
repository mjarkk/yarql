package graphql

import (
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestRegisterEnum(t *testing.T) {
	type TestEnumString string
	res := registerEnumCheck(map[string]TestEnumString{
		"A": "B",
	})
	NotNil(t, res)

	type TestEnumUint uint
	res = registerEnumCheck(map[string]TestEnumUint{
		"A": 1,
	})
	NotNil(t, res)

	type TestEnumInt uint
	res = registerEnumCheck(map[string]TestEnumInt{
		"A": 1,
	})
	NotNil(t, res)
}

func TestEmptyEnumShouldNotBeRegistered(t *testing.T) {
	type TestEnum string
	res := registerEnumCheck(map[string]TestEnum{})
	Nil(t, res)
}

func TestRegisterEnumFails(t *testing.T) {
	type TestEnum string

	Panics(t, func() {
		registerEnumCheck(0)
	}, "Cannot generate an enum of non map types")

	Panics(t, func() {
		registerEnumCheck(nil)
	}, "Cannot generate an enum of non map types 2")

	Panics(t, func() {
		registerEnumCheck(map[int]TestEnum{
			1: "a",
		})
	}, "Enum must have a string key type")

	Panics(t, func() {
		registerEnumCheck(map[string]struct{}{
			"a": {},
		})
	}, "Enum value cannot be complex")

	Panics(t, func() {
		registerEnumCheck(map[string]string{
			"foo": "bar",
		})
	}, "Enum value must be a custom type")

	Panics(t, func() {
		registerEnumCheck(map[string]TestEnum{
			"": "",
		})
	}, "Enum keys cannot be empty")

	// Maybe fix this??
	// Panics(t, func() {
	// 	registerEnumCheck(map[string]TestEnum{
	// 		"Foo": "Baz",
	// 		"Bar": "Baz",
	// 	})
	// }, "Enum cannot have duplicated values")

	Panics(t, func() {
		registerEnumCheck(map[string]TestEnum{
			"1": "",
		})
	}, "Enum cannot have an invalid graphql name, where first letter is number")

	Panics(t, func() {
		registerEnumCheck(map[string]TestEnum{
			"_": "",
		})
	}, "Enum cannot have an invalid graphql name, where first letter is underscore")

	Panics(t, func() {
		registerEnumCheck(map[string]TestEnum{
			"A!!!!": "",
		})
	}, "Enum cannot have an invalid graphql name, where remainder of name is invalid")
}

type TestEnum2 uint8

const (
	TestEnum2Foo TestEnum2 = iota
	TestEnum2Bar
	TestEnum2Baz
)

type TestEnumFunctionInput struct{}

func (TestEnumFunctionInput) ResolveBar(args struct{ E TestEnum2 }) TestEnum2 {
	return args.E
}

var testingRegisteredTestEnum = false

func TestEnum(t *testing.T) {
	var _ = RegisterEnum(map[string]TestEnum2{
		"FOO": TestEnum2Foo,
		"BAR": TestEnum2Bar,
		"BAZ": TestEnum2Baz,
	})
	testingRegisteredTestEnum = true

	out, errs := parseAndTest(t, `{bar(e: BAZ)}`, TestEnumFunctionInput{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"BAZ"}`, out)
}

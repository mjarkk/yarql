package graphql

import (
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestRegisterEnum(t *testing.T) {
	type TestEnumString string
	res, err := registerEnumCheck(map[string]TestEnumString{
		"A": "B",
	})
	NoError(t, err)
	NotNil(t, res)

	type TestEnumUint uint
	res, err = registerEnumCheck(map[string]TestEnumUint{
		"A": 1,
	})
	NoError(t, err)
	NotNil(t, res)

	type TestEnumInt uint
	res, err = registerEnumCheck(map[string]TestEnumInt{
		"A": 1,
	})
	NoError(t, err)
	NotNil(t, res)
}

func TestEmptyEnumShouldNotBeRegistered(t *testing.T) {
	type TestEnum string
	res, err := registerEnumCheck(map[string]TestEnum{})
	NoError(t, err)
	Nil(t, res)
}

func TestRegisterEnumFails(t *testing.T) {
	type TestEnum string

	_, err := registerEnumCheck(0)
	Error(t, err, "Cannot generate an enum of non map types")

	_, err = registerEnumCheck(nil)
	Error(t, err, "Cannot generate an enum of non map types 2")

	_, err = registerEnumCheck(map[int]TestEnum{1: "a"})
	Error(t, err, "Enum must have a string key type")

	_, err = registerEnumCheck(map[string]struct{}{"a": {}})
	Error(t, err, "Enum value cannot be complex")

	_, err = registerEnumCheck(map[string]string{"foo": "bar"})
	Error(t, err, "Enum value must be a custom type")

	_, err = registerEnumCheck(map[string]TestEnum{"": ""})
	Error(t, err, "Enum keys cannot be empty")

	// Maybe fix this??
	// _, err = registerEnumCheck(map[string]TestEnum{
	// 	"Foo": "Baz",
	// 	"Bar": "Baz",
	// })
	// Error(t, err, "Enum cannot have duplicated values")

	_, err = registerEnumCheck(map[string]TestEnum{"1": ""})
	Error(t, err, "Enum cannot have an invalid graphql name, where first letter is number")

	_, err = registerEnumCheck(map[string]TestEnum{"_": ""})
	Error(t, err, "Enum cannot have an invalid graphql name, where first letter is underscore")

	_, err = registerEnumCheck(map[string]TestEnum{"A!!!!": ""})
	Error(t, err, "Enum cannot have an invalid graphql name, where remainder of name is invalid")
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

func TestEnum(t *testing.T) {
	s := NewSchema()

	added, err := s.RegisterEnum(map[string]TestEnum2{
		"FOO": TestEnum2Foo,
		"BAR": TestEnum2Bar,
		"BAZ": TestEnum2Baz,
	})
	True(t, added)
	NoError(t, err)

	out, errs := parseAndTestWithOptions(
		t,
		s,
		`{bar(e: BAZ)}`,
		TestEnumFunctionInput{},
		M{},
		255,
		ResolveOptions{
			ReturnOnlyData: true,
		},
	)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"BAZ"}`, out)
}

package graphql

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestResolveSimpleVariable(t *testing.T) {
	// Normal variable
	variables := `{"baz": "foo"}`
	out, errs := parseAndTestWithOptions(t, `query($baz: String) {bar(a: $baz)}`, TestExecStructTypeMethodWithArgsData{}, M{}, 255, "", variables)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo"}`, out)

	// Default variable
	variables = `{}`
	out, errs = parseAndTestWithOptions(t, `query($baz: String = "foo") {bar(a: $baz)}`, TestExecStructTypeMethodWithArgsData{}, M{}, 255, "", variables)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo"}`, out)

	// Default variable and set variable
	variables = `{"baz": "FOOBAR"}`
	out, errs = parseAndTestWithOptions(t, `query($baz: String = "foo") {bar(a: $baz)}`, TestExecStructTypeMethodWithArgsData{}, M{}, 255, "", variables)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"FOOBAR"}`, out)
}

type TestResolveOtherSimpleVariableData struct{}

func (TestResolveOtherSimpleVariableData) ResolveBoolean(c *Ctx, args struct{ A bool }) bool {
	return args.A
}

func (TestResolveOtherSimpleVariableData) ResolveFloat(c *Ctx, args struct{ A float64 }) float64 {
	return args.A
}

func (TestResolveOtherSimpleVariableData) ResolveInt(c *Ctx, args struct{ A int }) int {
	return args.A
}

func TestResolveOtherSimpleVariable(t *testing.T) {
	tests := []struct {
		type_ string
		value string
	}{
		{"Boolean", "true"},
		{"Float", "1.1"},
		{"Int", "2"},
	}

	for _, test := range tests {
		field := strings.ToLower(test.type_)

		// Normal variable
		variables := fmt.Sprintf(`{"baz": %s}`, test.value)
		query := fmt.Sprintf(`query($baz: %s) {%s(a: $baz)}`, test.type_, field)
		out, errs := parseAndTestWithOptions(t, query, TestResolveOtherSimpleVariableData{}, M{}, 255, "", variables)
		for _, err := range errs {
			panic(err)
		}
		Equal(t, fmt.Sprintf(`{"%s":%s}`, field, test.value), out)

		// Using default variable
		variables = `{}`
		query = fmt.Sprintf(`query($baz: %s = %s) {%s(a: $baz)}`, test.type_, test.value, field)
		out, errs = parseAndTestWithOptions(t, query, TestResolveOtherSimpleVariableData{}, M{}, 255, "", variables)
		for _, err := range errs {
			panic(err)
		}
		Equal(t, fmt.Sprintf(`{"%s":%s}`, field, test.value), out)
	}

}

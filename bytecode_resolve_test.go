package graphql

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mjarkk/go-graphql/bytecode"
	. "github.com/stretchr/testify/assert"
)

func bytecodeParse(t *testing.T, query string, queries interface{}, methods interface{}) (string, []error) {
	s, err := ParseSchema(queries, methods, nil)
	NoError(t, err, query)

	ctx := BytecodeCtx{
		schema: s,
		query: bytecode.ParserCtx{
			Res:    []byte{},
			Query:  []byte{},
			Errors: []error{},
		},
		result:                 []byte{},
		charNr:                 0,
		reflectValues:          [256]reflect.Value{},
		currentReflectValueIdx: 0,
	}
	bytes, errs := ctx.BytecodeResolve([]byte(query), BytecodeParseOptions{NoMeta: true})
	return string(bytes), errs
}

func bytecodeParseAndExpectNoErrs(t *testing.T, query string, queries interface{}, methods interface{}) string {
	res, errs := bytecodeParse(t, query, queries, methods)
	for _, err := range errs {
		panic(err.Error())
	}
	return res
}

func TestBytecodeResolveOnlyOperation(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{}`, TestExecEmptyQueryDataQ{}, M{})
	Equal(t, `{}`, res)
}

func TestBytecodeResolveSingleField(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{a}`, TestExecSimpleQueryData{
		A: "foo",
		B: "bar",
	}, M{})
	Equal(t, `{"a":"foo"}`, res)
}

func TestBytecodeResolveMultipleFields(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{
		a,
		b,
	}`, TestExecSimpleQueryData{
		A: "foo",
		B: "bar",
	}, M{})
	Equal(t, `{"a":"foo","b":"bar"}`, res)
}

func TestBytecodeResolveNestedFields(t *testing.T) {
	schema := TestExecStructInStructInlineData{}
	json.Unmarshal([]byte(`{"foo": {"a": "foo", "b": "bar", "c": "baz"}}`), &schema)

	out := bytecodeParseAndExpectNoErrs(t, `{foo{a b}}`, schema, M{})

	res := TestExecStructInStructInlineData{}
	err := json.Unmarshal([]byte(out), &res)
	NoError(t, err)
	Equal(t, "foo", res.Foo.A)
	Equal(t, "bar", res.Foo.B)
}

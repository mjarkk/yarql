package graphql

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func parseAndTest(t *testing.T, query string, queries interface{}, methods interface{}) (string, []error) {
	s, err := ParseSchema(queries, methods, SchemaOptions{})
	NoError(t, err, query)
	out, errs := s.Resolve(query, "")
	if !json.Valid([]byte(out)) {
		panic(fmt.Sprintf("query %s, returned invalid json: %s", query, out))
	}
	return out, errs
}

type TestExecEmptyQueryDataQ struct{}
type TestExecEmptyQueryDataM struct{}

func TestExecEmptyQuery(t *testing.T) {
	_, errs := parseAndTest(t, `{}`, TestExecEmptyQueryDataQ{}, TestExecEmptyQueryDataM{})
	for _, err := range errs {
		panic(err)
	}
}

type TestExecSimpleQueryData struct {
	A string
	B string
	C string
}

func TestExecSimpleQuery(t *testing.T) {
	out, errs := parseAndTest(t, `{
		a
		b
	}`, TestExecSimpleQueryData{A: "foo", B: "bar", C: "baz"}, TestExecEmptyQueryDataM{})
	for _, err := range errs {
		panic(err)
	}

	res := map[string]string{}
	err := json.Unmarshal([]byte(out), &res)
	NoError(t, err)

	val, ok := res["a"]
	True(t, ok)
	Equal(t, "foo", val)
	val, ok = res["b"]
	True(t, ok)
	Equal(t, "bar", val)

	_, ok = res["c"]
	False(t, ok)
}

type TestExecStructInStructInlineData struct {
	Foo struct {
		A string `json:"a"`
		B string `json:"b"`
		C string `json:"c"`
	} `json:"foo"`
}

func TestExecStructInStructInline(t *testing.T) {
	schema := TestExecStructInStructInlineData{}
	json.Unmarshal([]byte(`{"foo": {"a": "foo", "b": "bar", "c": "baz"}}`), &schema)

	out, errs := parseAndTest(t, `{
		foo {
			a
			b
		}
	}`, schema, TestExecEmptyQueryDataM{})
	for _, err := range errs {
		panic(err)
	}

	res := TestExecStructInStructInlineData{}
	err := json.Unmarshal([]byte(out), &res)
	NoError(t, err)

	Equal(t, "foo", res.Foo.A)
	Equal(t, "bar", res.Foo.B)
}

type TestExecStructInStructData struct {
	Foo TestExecSimpleQueryData
}

func TestExecStructInStruct(t *testing.T) {
	out, errs := parseAndTest(t, `{
		foo {
			a
			b
		}
	}`, TestExecStructInStructData{
		Foo: TestExecSimpleQueryData{
			A: "foo",
			B: "bar",
			C: "baz",
		},
	}, TestExecEmptyQueryDataM{})
	for _, err := range errs {
		panic(err)
	}

	res := TestExecStructInStructInlineData{}
	err := json.Unmarshal([]byte(out), &res)
	NoError(t, err)

	Equal(t, "foo", res.Foo.A)
	Equal(t, "bar", res.Foo.B)
}

func TestExecInvalidFields(t *testing.T) {
	out, errs := parseAndTest(t, `{field_that_does_not_exsist{a b}}`, TestExecStructInStructData{}, TestExecEmptyQueryDataM{})
	Equal(t, 1, len(errs), "Response should have errors")
	Equal(t, "{}", out, "response should be empty")

	out, errs = parseAndTest(t, `{foo{field_that_does_not_exsist}}`, TestExecStructInStructData{}, TestExecEmptyQueryDataM{})
	Equal(t, 1, len(errs), "Response should have errors")
	Equal(t, "{}", out, "response should be empty")
}

func TestExecAlias(t *testing.T) {
	out, errs := parseAndTest(t, `{
		aa: a
		ba: a
		ca: a

		ab: b
		bb: b
		cb: b

		ac: c
		bc: c
		cc: c
	}`, TestExecSimpleQueryData{
		A: "foo",
		B: "bar",
		C: "baz",
	}, TestExecEmptyQueryDataM{})
	for _, err := range errs {
		panic(err)
	}

	res := map[string]string{}
	err := json.Unmarshal([]byte(out), &res)
	NoError(t, err)

	tests := []struct {
		expect string
		for_   []string
	}{
		{"foo", []string{"aa", "ba", "ca"}},
		{"bar", []string{"ab", "bb", "cb"}},
		{"baz", []string{"ac", "bc", "cc"}},
	}

	for _, test := range tests {
		for _, item := range test.for_ {
			Equal(t, test.expect, res[item], fmt.Sprintf("Expect %s to be %s", item, test.expect))
		}
	}
}

package graphql

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestValueToJson(t *testing.T) {
	string_ := string(`a"b`)
	boolTrue := bool(true)
	boolFalse := bool(false)
	int_ := int(1)
	int8_ := int8(2)
	int16_ := int16(3)
	int32_ := int32(4)
	int64_ := int64(5)
	uint_ := uint(6)
	uint8_ := uint8(7)
	uint16_ := uint16(8)
	uint32_ := uint32(9)
	uint64_ := uint64(10)
	uintptr_ := uintptr(11)
	float32_ := float32(12)
	float64_ := float64(13)
	float64WExponent := 100e-100

	var stringPtr *string
	var boolPtr *bool
	var intPtr *int
	var int8Ptr *int8
	var int16Ptr *int16
	var int32Ptr *int32
	var int64Ptr *int64
	var uintPtr *uint
	var uint8Ptr *uint8
	var uint16Ptr *uint16
	var uint32Ptr *uint32
	var uint64Ptr *uint64
	var uintptrPtr *uintptr
	var float32Ptr *float32
	var float64Ptr *float64

	options := []struct {
		value  interface{}
		expect string
	}{
		{string_, `"a\"b"`},
		{boolTrue, "true"},
		{boolFalse, "false"},
		{int_, "1"},
		{int8_, "2"},
		{int16_, "3"},
		{int32_, "4"},
		{int64_, "5"},
		{uint_, "6"},
		{uint8_, "7"},
		{uint16_, "8"},
		{uint32_, "9"},
		{uint64_, "10"},
		{uintptr_, "11"},
		{float32_, "12"},
		{float64_, "13"},
		{float64WExponent, "1e-98"},

		{&string_, `"a\"b"`},
		{&boolTrue, "true"},
		{&boolFalse, "false"},
		{&int_, "1"},
		{&int8_, "2"},
		{&int16_, "3"},
		{&int32_, "4"},
		{&int64_, "5"},
		{&uint_, "6"},
		{&uint8_, "7"},
		{&uint16_, "8"},
		{&uint32_, "9"},
		{&uint64_, "10"},
		{&uintptr_, "11"},
		{&float32_, "12"},
		{&float64_, "13"},

		{stringPtr, `null`},
		{boolPtr, "null"},
		{intPtr, "null"},
		{int8Ptr, "null"},
		{int16Ptr, "null"},
		{int32Ptr, "null"},
		{int64Ptr, "null"},
		{uintPtr, "null"},
		{uint8Ptr, "null"},
		{uint16Ptr, "null"},
		{uint32Ptr, "null"},
		{uint64Ptr, "null"},
		{uintptrPtr, "null"},
		{float32Ptr, "null"},
		{float64Ptr, "null"},

		{complex64(1), "null"},
	}
	for _, option := range options {
		res, _ := valueToJson(option.value)
		Equal(t, option.expect, res)
	}
}

func parseAndTest(t *testing.T, query string, queries interface{}, methods interface{}) (string, []error) {
	return parseAndTestMaxDeptAndOperatorTarget(t, query, queries, methods, 255, "")
}

func parseAndTestMaxDeptAndOperatorTarget(t *testing.T, query string, queries interface{}, methods interface{}, maxDept uint8, operatorTarget string) (string, []error) {
	s, err := ParseSchema(queries, methods, SchemaOptions{})
	NoError(t, err, query)
	s.MaxDepth = maxDept
	out, errs := s.Resolve(query, operatorTarget)
	if !json.Valid([]byte(out)) {
		panic(fmt.Sprintf("query %s, returned invalid json: %s", query, out))
	}
	return out, errs
}

type TestExecEmptyQueryDataQ struct{}
type M struct{}

func TestExecEmptyQuery(t *testing.T) {
	_, errs := parseAndTest(t, `{}`, TestExecEmptyQueryDataQ{}, M{})
	for _, err := range errs {
		panic(err)
	}
}

type TestExecSimpleQueryData struct {
	A string
	B string
	C string
	D string
}

func TestExecSimpleQuery(t *testing.T) {
	out, errs := parseAndTest(t, `{
		a
		b
	}`, TestExecSimpleQueryData{A: "foo", B: "bar", C: "baz"}, M{})
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
	}`, schema, M{})
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
	}, M{})
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
	out, errs := parseAndTest(t, `{field_that_does_not_exsist{a b}}`, TestExecStructInStructData{}, M{})
	Equal(t, 1, len(errs), "Response should have errors")
	Equal(t, "{}", out, "response should be empty")

	out, errs = parseAndTest(t, `{foo{field_that_does_not_exsist}}`, TestExecStructInStructData{}, M{})
	Equal(t, 1, len(errs), "Response should have errors")
	Equal(t, `{"foo":{}}`, out, "response should be empty")
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
	}, M{})
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

type TestExecArrayData struct {
	Foo []string
}

func TestExecArray(t *testing.T) {
	out, errs := parseAndTest(t, `{foo}`, TestExecArrayData{[]string{"a", "b", "c"}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":["a","b","c"]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayData{[]string{}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayData{nil}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":null}`, out)
}

type TestExecArrayWithStructData struct {
	Foo []TestExecSimpleQueryData
}

func TestExecArrayWithStruct(t *testing.T) {
	out, errs := parseAndTest(t, `{foo {a b}}`, TestExecArrayWithStructData{[]TestExecSimpleQueryData{{}}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[{"a":"","b":""}]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayWithStructData{[]TestExecSimpleQueryData{}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayWithStructData{nil}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":null}`, out)
}

type TestExecArrayWithinArrayData struct {
	Foo [][]string
}

func TestExecArrayWithinArray(t *testing.T) {
	out, errs := parseAndTest(t, `{foo}`, TestExecArrayWithinArrayData{[][]string{{"a", "b", "c"}}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[["a","b","c"]]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayWithinArrayData{[][]string{{"a"}, {"b"}, {"c"}}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[["a"],["b"],["c"]]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayWithinArrayData{[][]string{{}}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[[]]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayWithinArrayData{[][]string{}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayWithinArrayData{[][]string{nil}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[null]}`, out)

	out, errs = parseAndTest(t, `{foo}`, TestExecArrayWithinArrayData{nil}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":null}`, out)
}

type TestExecPtrData struct {
	Foo *string
}

func TestExecPtr(t *testing.T) {
	out, errs := parseAndTest(t, `{foo}`, TestExecPtrData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":null}`, out)

	data := "bar"
	out, errs = parseAndTest(t, `{foo}`, TestExecPtrData{&data}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":"bar"}`, out)
}

type TestExecPtrInPtrData struct {
	Foo **string
}

func TestExecPtrInPtr(t *testing.T) {
	out, errs := parseAndTest(t, `{foo}`, TestExecPtrInPtrData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":null}`, out)

	data := "bar"
	dataPtr := &data
	out, errs = parseAndTest(t, `{foo}`, TestExecPtrInPtrData{&dataPtr}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":"bar"}`, out)
}

type TestExecArrayWithPtrData struct {
	Foo []*TestExecSimpleQueryData
}

func TestExecArrayWithPtr(t *testing.T) {
	out, errs := parseAndTest(t, `{foo{a b}}`, TestExecArrayWithPtrData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":null}`, out)

	out, errs = parseAndTest(t, `{foo{a b}}`, TestExecArrayWithPtrData{[]*TestExecSimpleQueryData{}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[]}`, out)

	out, errs = parseAndTest(t, `{foo{a b}}`, TestExecArrayWithPtrData{[]*TestExecSimpleQueryData{nil}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[null]}`, out)

	out, errs = parseAndTest(t, `{foo{a b}}`, TestExecArrayWithPtrData{[]*TestExecSimpleQueryData{{A: "foo", B: "bar", C: "baz"}}}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":[{"a":"foo","b":"bar"}]}`, out)
}

type TestExecMaxDeptData struct {
	Foo struct {
		Bar struct {
			Baz struct {
				FooBar struct {
					BarBaz struct {
						BazFoo string
					}
				}
			}
		}
	}
}

func TestExecMaxDept(t *testing.T) {
	out, errs := parseAndTestMaxDeptAndOperatorTarget(t, `{foo{bar{baz{fooBar{barBaz{bazFoo}}}}}}`, TestExecMaxDeptData{}, M{}, 3, "")
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":{"bar":{"baz":null}}}`, out)
}

type TestExecStructMethodData struct {
	Foo func() string
}

func TestExecStructMethod(t *testing.T) {
	out, errs := parseAndTest(t, `{foo}`, TestExecStructMethodData{
		Foo: func() string { return "bar" },
	}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":"bar"}`, out)
}

type TestExecStructTypeMethodData struct{}

func (TestExecStructTypeMethodData) ResolveBar() string {
	return "foo"
}

func (TestExecStructTypeMethodData) ResolveBaz() (string, error) {
	return "bar", nil
}

func TestExecStructTypeMethod(t *testing.T) {
	out, errs := parseAndTest(t, `{bar, baz}`, TestExecStructTypeMethodData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo","baz":"bar"}`, out)
}

type TestExecStructTypeMethodWithCtxData struct{}

func (TestExecStructTypeMethodWithCtxData) ResolveBar(c *Ctx) string {
	c.Values["baz"] = "bar"
	return "foo"
}

func (TestExecStructTypeMethodWithCtxData) ResolveBaz(c *Ctx) (string, error) {
	value, ok := c.Values["baz"]
	if !ok {
		return "", errors.New("baz not set by bar resolver")
	}
	return value.(string), nil
}

func TestExecStructTypeMethodWithCtx(t *testing.T) {
	out, errs := parseAndTest(t, `{bar, baz}`, TestExecStructTypeMethodData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo","baz":"bar"}`, out)
}

type TestExecStructTypeMethodWithArgsData struct{}

func (TestExecStructTypeMethodWithArgsData) ResolveBar(c *Ctx, args struct{ A string }) string {
	return args.A
}

func TestExecStructTypeMethodWithArgs(t *testing.T) {
	out, errs := parseAndTest(t, `{bar(a: "foo")}`, TestExecStructTypeMethodWithArgsData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo"}`, out)
}

type TestExecStructTypeMethodWithListArgData struct{}

func (TestExecStructTypeMethodWithListArgData) ResolveBar(args struct{ A []string }) []string {
	return args.A
}

func TestExecStructTypeMethodWithListArg(t *testing.T) {
	out, errs := parseAndTest(t, `{bar(a: [])}`, TestExecStructTypeMethodWithListArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":[]}`, out)

	out, errs = parseAndTest(t, `{bar()}`, TestExecStructTypeMethodWithListArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":null}`, out)

	out, errs = parseAndTest(t, `{bar(a: ["foo","bar"])}`, TestExecStructTypeMethodWithListArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":["foo","bar"]}`, out)
}

func TestExecInlineFragment(t *testing.T) {
	out, errs := parseAndTest(t, `{a...{b, c} d}`, TestExecSimpleQueryData{A: "foo", B: "bar", C: "baz", D: "foobar"}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"a":"foo","b":"bar","c":"baz","d":"foobar"}`, out)
}

func TestExecFragment(t *testing.T) {
	query := `
	fragment BAndCFrag on Something{b c}

	query {a...BAndCFrag d}
	`

	out, errs := parseAndTest(t, query, TestExecSimpleQueryData{A: "foo", B: "bar", C: "baz", D: "foobar"}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"a":"foo","b":"bar","c":"baz","d":"foobar"}`, out)
}

func TestExecMultipleOperators(t *testing.T) {
	query := `
	query QueryA {a b}
	query QueryB {c d}
	`
	out, errs := parseAndTestMaxDeptAndOperatorTarget(t, query, TestExecSimpleQueryData{}, M{}, 255, "")
	Equal(t, 1, len(errs))
	Equal(t, "{}", out)

	out, errs = parseAndTestMaxDeptAndOperatorTarget(t, query, TestExecSimpleQueryData{}, M{}, 255, "QueryA")
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"a":"","b":""}`, out)

	out, errs = parseAndTestMaxDeptAndOperatorTarget(t, query, TestExecSimpleQueryData{}, M{}, 255, "QueryB")
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"c":"","d":""}`, out)
}

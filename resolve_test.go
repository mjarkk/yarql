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
	s, err := ParseSchema(queries, methods, nil)
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
	out, errs := parseAndTest(t, `{a b}`, TestExecSimpleQueryData{A: "foo", B: "bar", C: "baz"}, M{})
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

func TestGenerateResponse(t *testing.T) {
	out, errs := parseAndTest(t, `{
		a
		b
		non_exsisting_field
	}`, TestExecSimpleQueryData{A: "foo", B: "bar", C: "baz"}, M{})
	res := GenerateResponse(out, errs)
	Equal(t, `{"data":{"a":"foo","b":"bar"}"errors":[{"message":"field non_exsisting_field does not exists on TestExecSimpleQueryData"}]}`, res)
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

	out, errs := parseAndTest(t, `{foo{a b}}`, schema, M{})
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
	Greater(t, len(errs), 0)
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

type TestExecStructTypeMethodData struct {
	Foo func() string
}

func (TestExecStructTypeMethodData) ResolveBar() string {
	return "foo"
}

func (TestExecStructTypeMethodData) ResolveBaz() (string, error) {
	return "bar", nil
}

func TestExecStructTypeMethod(t *testing.T) {
	out, errs := parseAndTest(t, `{foo, bar, baz}`, TestExecStructTypeMethodData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":null,"bar":"foo","baz":"bar"}`, out)
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

type TestExecStructTypeMethodWithStructArgData struct{}

func (TestExecStructTypeMethodWithStructArgData) ResolveBar(c *Ctx, args struct{ A struct{ B string } }) string {
	return args.A.B
}

func TestExecStructTypeMethodWithStructArg(t *testing.T) {
	out, errs := parseAndTest(t, `{bar(a: {b: "foo"})}`, TestExecStructTypeMethodWithStructArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo"}`, out)
}

type TestExecStructTypeMethodWithPtrArgData struct{}

func (TestExecStructTypeMethodWithPtrArgData) ResolveBar(c *Ctx, args struct{ A *string }) *string {
	return args.A
}

func TestExecStructTypeMethodWithPtrArg(t *testing.T) {
	out, errs := parseAndTest(t, `{bar()}`, TestExecStructTypeMethodWithPtrArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":null}`, out)

	out, errs = parseAndTest(t, `{bar(a: null)}`, TestExecStructTypeMethodWithPtrArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":null}`, out)

	out, errs = parseAndTest(t, `{bar(a: "foo")}`, TestExecStructTypeMethodWithPtrArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo"}`, out)
}

type TestExecStructTypeMethodWithPtrInPtrArgData struct{}

func (TestExecStructTypeMethodWithPtrInPtrArgData) ResolveBar(c *Ctx, args struct{ A **string }) **string {
	return args.A
}

func TestExecStructTypeMethodWithPtrInPtrArg(t *testing.T) {
	out, errs := parseAndTest(t, `{bar()}`, TestExecStructTypeMethodWithPtrInPtrArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":null}`, out)

	out, errs = parseAndTest(t, `{bar(a: null)}`, TestExecStructTypeMethodWithPtrInPtrArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":null}`, out)

	out, errs = parseAndTest(t, `{bar(a: "foo")}`, TestExecStructTypeMethodWithPtrInPtrArgData{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":"foo"}`, out)
}

type TestExecStructTypeMethodWithStructArgNPlus1Data struct{}

type TestNPlus1Input struct {
	Ptr *TestNPlus1Input
	Arr []TestNPlus1Input
}

func (TestExecStructTypeMethodWithStructArgNPlus1Data) ResolveBar(c *Ctx, args struct{ A TestNPlus1Input }) []TestNPlus1Input {
	return args.A.Ptr.Ptr.Arr
}

func TestExecStructTypeMethodWithStructArgNPlus1(t *testing.T) {
	out, errs := parseAndTest(t, `{bar(a: {ptr: {ptr: {arr: []}}})}`, TestExecStructTypeMethodWithStructArgNPlus1Data{}, M{})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"bar":[]}`, out)
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

// This is the request graphql playground makes to get the schema
var schemaQuery = `
query IntrospectionQuery {
	__schema {
		queryType {
			name
		}
		mutationType {
			name
		}
		subscriptionType {
			name
		}
		types {
			...FullType
		}
		directives {
			name
			description
			locations
			args {
				...InputValue
			}
		}
	}
}

fragment FullType on __Type {
	kind
	name
	description
	fields(includeDeprecated: true) {
		name
		description
		args {
			...InputValue
		}
		type {
			...TypeRef
		}
		isDeprecated
		deprecationReason
	}
	inputFields {
		...InputValue
	}
	interfaces {
		...TypeRef
	}
	enumValues(includeDeprecated: true) {
		name
		description
		isDeprecated
		deprecationReason
	}
	possibleTypes {
		...TypeRef
	}
}

fragment InputValue on __InputValue {
	name
	description
	type {
		...TypeRef
	}
	defaultValue
}

fragment TypeRef on __Type {
	kind
	name
	ofType {
		kind
		name
		ofType {
			kind
			name
			ofType {
				kind
				name
				ofType {
					kind
					name
					ofType {
						kind
						name
						ofType {
							kind
							name
							ofType {
								kind
								name
							}
						}
					}
				}
			}
		}
	}
}
`

type TestExecSchemaRequestSimpleData struct{}

func TestExecSchemaRequestSimple(t *testing.T) {
	resString, errs := parseAndTest(t, schemaQuery, TestExecSchemaRequestSimpleData{}, M{})
	for _, err := range errs {
		panic(err)
	}

	res := struct {
		Schema QLSchema `json:"__schema"`
	}{}
	err := json.Unmarshal([]byte(resString), &res)
	NoError(t, err)

	schema := res.Schema
	types := schema.Types
	Equal(t, 12, len(types))

	idx := 0
	is := func(kind, name string) {
		item := types[idx]
		Equalf(t, kind, item.Kind, "(KIND) Index: %d", idx)
		NotNilf(t, item.Name, "(NAME) Index: %d", idx)
		Equalf(t, name, *item.Name, "(NAME) Index: %d", idx)
		idx++
	}

	is("OBJECT", "M")                               // 0
	is("OBJECT", "TestExecSchemaRequestSimpleData") // 1
	is("OBJECT", "__Directive")                     // 2
	is("OBJECT", "__EnumValue")                     // 3
	is("OBJECT", "__Field")                         // 4
	is("OBJECT", "__InputValue")                    // 5
	is("OBJECT", "__Schema")                        // 6
	is("OBJECT", "__Type")                          // 7
	is("SCALAR", "Boolean")                         // 8
	is("SCALAR", "Int")                             // 9
	is("SCALAR", "Float")                           // 10
	is("SCALAR", "String")                          // 11
}

type TestExecSchemaRequestWithFieldsDataInnerStruct struct {
	Foo *string
	Bar string
}

type TestExecSchemaRequestWithFieldsData struct {
	A TestExecSchemaRequestWithFieldsDataInnerStruct
	B struct {
		Baz string
	}
	C struct {
		FooBar []TestExecSchemaRequestWithFieldsDataInnerStruct
	}
}

func (TestExecSchemaRequestWithFieldsData) ResolveD(args struct{ Foo struct{ Bar string } }) TestExecSchemaRequestWithFieldsDataInnerStruct {
	return TestExecSchemaRequestWithFieldsDataInnerStruct{}
}

func TestExecSchemaRequestWithFields(t *testing.T) {
	resString, errs := parseAndTest(t, schemaQuery, TestExecSchemaRequestWithFieldsData{}, M{})
	for _, err := range errs {
		panic(err)
	}

	res := struct {
		Schema QLSchema `json:"__schema"`
	}{}
	err := json.Unmarshal([]byte(resString), &res)
	NoError(t, err)

	schema := res.Schema
	types := schema.Types
	Equal(t, 16, len(types))

	idx := 0
	is := func(kind, name string) {
		item := types[idx]
		Equalf(t, kind, item.Kind, "(KIND) Index: %d", idx)
		NotNilf(t, item.Name, "(NAME) Index: %d", idx)
		Equalf(t, name, *item.Name, "(NAME) Index: %d", idx)
		idx++
	}

	is("OBJECT", "M") // 0
	queryIdx := 1
	is("OBJECT", "TestExecSchemaRequestWithFieldsData")            // 1
	is("OBJECT", "TestExecSchemaRequestWithFieldsDataInnerStruct") // 2
	is("OBJECT", "__Directive")                                    // 3
	is("OBJECT", "__EnumValue")                                    // 4
	is("OBJECT", "__Field")                                        // 5
	is("OBJECT", "__InputValue")                                   // 6
	is("OBJECT", "__Schema")                                       // 7
	is("OBJECT", "__Type")                                         // 8
	is("INPUT_OBJECT", "__UnknownInput1")                          // 9
	is("OBJECT", "__UnknownType1")                                 // 10
	is("OBJECT", "__UnknownType2")                                 // 11
	is("SCALAR", "Boolean")                                        // 12
	is("SCALAR", "Int")                                            // 13
	is("SCALAR", "Float")                                          // 14
	is("SCALAR", "String")                                         // 15

	fields := types[queryIdx].JSONFields
	Equal(t, 5, len(fields))

	idx = 0
	isField := func(name string) {
		field := fields[idx]
		Equalf(t, name, field.Name, "(NAME) Index: %d", idx)
		Equalf(t, "NON_NULL", field.Type.Kind, "(KIND) Index: %d", idx)
		Equalf(t, "OBJECT", field.Type.OfType.Kind, "(OFTYPE KIND) Index: %d", idx)
		idx++
	}

	isField("__schema")
	isField("a")
	isField("b")
	isField("c")
	isField("d")
}

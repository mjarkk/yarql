package graphql

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mjarkk/go-graphql/helpers"
	. "github.com/stretchr/testify/assert"
)

func bytecodeParse(t *testing.T, s *Schema, query string, queries interface{}, methods interface{}, opts ...ResolveOptions) (string, []error) {
	err := s.Parse(queries, methods, nil)
	NoError(t, err, query)

	if len(opts) == 0 {
		opts = []ResolveOptions{{NoMeta: true}}
	}
	errs := s.Resolve([]byte(query), opts[0])
	return string(s.Result), errs
}

func bytecodeParseAndExpectNoErrs(t *testing.T, query string, queries interface{}, methods interface{}, opts ...ResolveOptions) string {
	res, errs := bytecodeParse(t, NewSchema(), query, queries, methods, opts...)
	for _, err := range errs {
		panic(err.Error())
	}
	return res
}

func bytecodeParseAndExpectErrs(t *testing.T, query string, queries interface{}, methods interface{}, opts ...ResolveOptions) (string, []error) {
	res, errs := bytecodeParse(t, NewSchema(), query, queries, methods, opts...)
	NotEqual(t, 0, len(res), query)
	return res, errs
}

type TestResolveEmptyQueryDataQ struct{}
type M struct{}

func TestBytecodeResolveOnlyOperation(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{}`, TestResolveEmptyQueryDataQ{}, M{})
	Equal(t, `{}`, res)
}

type TestResolveSimpleQueryData struct {
	A string
	B string
	C string
	D string
}

func TestBytecodeResolveSingleField(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{a}`, TestResolveSimpleQueryData{
		A: "foo",
		B: "bar",
	}, M{})
	Equal(t, `{"a":"foo"}`, res)
}

func TestBytecodeResolveMutation(t *testing.T) {
	schema := TestResolveSimpleQueryData{
		A: "foo",
		B: "bar",
	}
	res := bytecodeParseAndExpectNoErrs(t, `mutation test {a}`, M{}, schema)
	Equal(t, `{"a":"foo"}`, res)

	_, errs := bytecodeParseAndExpectErrs(t, `mutation test {a}`, schema, M{})
	Len(t, errs, 1)
}

func TestBytecodeResolveMultipleFields(t *testing.T) {
	schema := TestResolveSimpleQueryData{
		A: "foo",
		B: "bar",
	}
	res := bytecodeParseAndExpectNoErrs(t, `{
		a,
		b,
	}`, schema, M{})
	Equal(t, `{"a":"foo","b":"bar"}`, res)
}

func TestBytecodeResolveAlias(t *testing.T) {
	schema := TestResolveSimpleQueryData{
		A: "foo",
		B: "bar",
	}
	res := bytecodeParseAndExpectNoErrs(t, `{b:a}`, schema, M{})
	Equal(t, `{"b":"foo"}`, res)
}

func TestBytecodeResolveOperatorWithName(t *testing.T) {
	schema := TestResolveSimpleQueryData{
		A: "foo",
		B: "bar",
	}

	testCases := []struct {
		target         string
		expectedResult string
	}{
		{"", `{"b":"bar"}`},
		{"a", `{"a":"foo"}`},
		{"b", `{"b":"bar"}`},
	}

	for _, testCase := range testCases {
		t.Run("target "+testCase.target, func(t *testing.T) {
			res, errs := bytecodeParse(t, NewSchema(), `query a {a} query b {b}`, schema, M{}, ResolveOptions{
				NoMeta:         true,
				OperatorTarget: testCase.target,
			})
			for _, err := range errs {
				panic(err)
			}
			Equal(t, testCase.expectedResult, res)
		})
	}
}

type TestResolveStructInStructInlineData struct {
	Foo struct {
		A string `json:"a"`
		B string `json:"b"`
		C string `json:"c"`
	} `json:"foo"`
}

func TestBytecodeResolveNestedFields(t *testing.T) {
	schema := TestResolveStructInStructInlineData{}
	json.Unmarshal([]byte(`{"foo": {"a": "foo", "b": "bar", "c": "baz"}}`), &schema)

	out := bytecodeParseAndExpectNoErrs(t, `{foo{a b}}`, schema, M{})

	res := TestResolveStructInStructInlineData{}
	err := json.Unmarshal([]byte(out), &res)
	NoError(t, err)
	Equal(t, "foo", res.Foo.A)
	Equal(t, "bar", res.Foo.B)
}

func TestBytecodeResolveMultipleNestedFields(t *testing.T) {
	schema := TestResolveSchemaRequestWithFieldsData{}
	res := bytecodeParseAndExpectNoErrs(t, `{
		a {
			foo
			bar
		}
		b {
			baz
		}
	}`, schema, M{})
	Equal(t, `{"a":{"foo":null,"bar":""},"b":{"baz":""}}`, res)
}

type TestResolveArrayData struct {
	Foo []string
}

func TestBytecodeResolveArray(t *testing.T) {
	schema := TestResolveArrayData{
		Foo: []string{"foo", "bar"},
	}
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, schema, M{})
	Equal(t, `{"foo":["foo","bar"]}`, res)
}

type TestBytecodeResolveStructsArrayData struct {
	Foo []TestResolveSimpleQueryData
}

func TestBytecodeResolveStructsArray(t *testing.T) {
	schema := TestBytecodeResolveStructsArrayData{
		Foo: []TestResolveSimpleQueryData{
			{A: "foo", B: "bar"},
			{A: "baz", B: "boz"},
		},
	}
	res := bytecodeParseAndExpectNoErrs(t, `{foo{a b}}`, schema, M{})
	Equal(t, `{"foo":[{"a":"foo","b":"bar"},{"a":"baz","b":"boz"}]}`, res)
}

type TestBytecodeResolveTimeData struct {
	T time.Time
}

func TestBytecodeResolveTime(t *testing.T) {
	now := time.Now()
	expect := []byte{}
	helpers.TimeToIso8601String(&expect, now)

	schema := TestBytecodeResolveTimeData{now}
	res := bytecodeParseAndExpectNoErrs(t, `{t}`, schema, M{})
	Equal(t, `{"t":"`+string(expect)+`"}`, res)
}

type TestResolveTimeIOData struct{}

func (TestResolveTimeIOData) ResolveFoo(args struct{ T time.Time }) time.Time {
	return args.T.AddDate(3, 2, 1).Add(time.Hour + time.Second)
}

func TestBytecodeResolveTimeIO(t *testing.T) {
	now := time.Now()
	testTimeInput := []byte{}
	helpers.TimeToIso8601String(&testTimeInput, now)

	query := `{foo(t: "` + string(testTimeInput) + `")}`
	out := bytecodeParseAndExpectNoErrs(t, query, TestResolveTimeIOData{}, M{})

	exectedOutTime := []byte{}
	helpers.TimeToIso8601String(&exectedOutTime, now.AddDate(3, 2, 1).Add(time.Hour+time.Second))
	Equal(t, `{"foo":"`+string(exectedOutTime)+`"}`, out)
}

type TestResolveStructTypeMethodData struct {
	Foo func() string
}

func (TestResolveStructTypeMethodData) ResolveBar() string {
	return "foo"
}

func (TestResolveStructTypeMethodData) ResolveBaz() (string, error) {
	return "bar", nil
}

func TestBytecodeResolveMethod(t *testing.T) {
	schema := TestResolveStructTypeMethodData{}
	res := bytecodeParseAndExpectNoErrs(t, `{foo, bar, baz}`, schema, M{})
	Equal(t, `{"foo":null,"bar":"foo","baz":"bar"}`, res)
}

type TestResolveStructTypeMethodWithArgsData struct{}

func (TestResolveStructTypeMethodWithArgsData) ResolveBar(c *Ctx, args struct{ A string }) string {
	return args.A
}

func TestBytecodeResolveMethodWithArg(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar(a: "foo")}`, TestResolveStructTypeMethodWithArgsData{}, M{})
	Equal(t, `{"bar":"foo"}`, res)
}

type TestResolveInputAllKindsOfNumbersData struct{}

func (TestResolveInputAllKindsOfNumbersData) ResolveFoo(args TestResolveInputAllKindsOfNumbersDataIO) TestResolveInputAllKindsOfNumbersDataIO {
	return args
}

type TestResolveInputAllKindsOfNumbersDataIO struct {
	A int8
	B uint8
	C float64
	D float32
}

func TestBytecodeResolveMethodWithIntArgs(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo(a: 1, b: 2, c: 3, d: 1.1) {a b c d}}`, TestResolveInputAllKindsOfNumbersData{}, M{})
	Equal(t, `{"foo":{"a":1,"b":2,"c":3,"d":1.1}}`, res)
}

func TestBytecodeResolveTypename(t *testing.T) {
	schema := TestResolveStructTypeMethodData{}
	res := bytecodeParseAndExpectNoErrs(t, `{__typename}`, schema, M{})
	Equal(t, `{"__typename":"TestResolveStructTypeMethodData"}`, res)
}

type TestResolvePtrData struct {
	Foo *string
}

type TestResolvePtrInPtrData struct {
	Foo **string
}

func TestBytecodeResolveOutputPointer(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrData{}, M{})
	Equal(t, `{"foo":null}`, res)

	data := "bar"
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrData{&data}, M{})
	Equal(t, `{"foo":"bar"}`, res)

	// Nested pointers
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrInPtrData{}, M{})
	Equal(t, `{"foo":null}`, res)

	ptrToData := &data
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrInPtrData{&ptrToData}, M{})
	Equal(t, `{"foo":"bar"}`, res)
}

type TestResolveStructTypeMethodWithPtrArgData struct{}

func (TestResolveStructTypeMethodWithPtrArgData) ResolveBar(c *Ctx, args struct{ A *string }) *string {
	return args.A
}

func TestBytecodeResolveMethodPointerInput(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar()}`, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: null)}`, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: "foo")}`, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":"foo"}`, res)
}

type TestBytecodeResolveMethodListInputData struct{}

func (TestBytecodeResolveMethodListInputData) ResolveBar(c *Ctx, args struct{ A []string }) []string {
	return args.A
}

func TestBytecodeResolveMethodListInput(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar()}`, TestBytecodeResolveMethodListInputData{}, M{})
	Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: null)}`, TestBytecodeResolveMethodListInputData{}, M{})
	Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: ["foo", "baz"])}`, TestBytecodeResolveMethodListInputData{}, M{})
	Equal(t, `{"bar":["foo","baz"]}`, res)
}

type TestResolveStructTypeMethodWithStructArgData struct{}

func (TestResolveStructTypeMethodWithStructArgData) ResolveBar(c *Ctx, args struct{ A struct{ B string } }) string {
	return args.A.B
}

func TestBytecodeResolveMethodNestedInputs(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar(a: {b: "foo"})}`, TestResolveStructTypeMethodWithStructArgData{}, M{})
	Equal(t, `{"bar":"foo"}`, res)
}

type TestBytecodeResolveEnumData struct {
	foo __TypeKind
}

func TestBytecodeResolveEnum(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, TestBytecodeResolveEnumData{
		foo: typeKindObject,
	}, M{})
	Equal(t, `{"foo":"OBJECT"}`, res)
}

type TestBytecodeResolveEnumInputData struct{}

func (TestBytecodeResolveEnumInputData) ResolveFoo(args struct{ A __TypeKind }) __TypeKind {
	return args.A
}

func TestBytecodeResolveEnumInput(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo(a: OBJECT)}`, TestBytecodeResolveEnumInputData{}, M{})
	Equal(t, `{"foo":"OBJECT"}`, res)
}

func TestBytecodeResolveCorrectMeta(t *testing.T) {
	query := `{
		a {
			foo
			bar
		}
		b {
			baz
		}
	}`
	schema := TestResolveSchemaRequestWithFieldsData{}
	res, _ := bytecodeParse(t, NewSchema(), query, schema, M{}, ResolveOptions{})
	if !json.Valid([]byte(res)) {
		panic("invalid json: " + res)
	}
	Equal(t, `{"data":{"a":{"foo":null,"bar":""},"b":{"baz":""}},"errors":[],"extensions":{}}`, res)
}

func TestBytecodeResolveCorrectMetaWithError(t *testing.T) {
	query := `{
		a {
			foo(a: "")
		}
	}`
	schema := TestResolveSchemaRequestWithFieldsData{}
	res, _ := bytecodeParse(t, NewSchema(), query, schema, M{}, ResolveOptions{})
	if !json.Valid([]byte(res)) {
		panic("invalid json: " + res)
	}
	Equal(t, `{"data":{"a":{"foo":null}},"errors":[{"message":"field arguments not allowed","path":["a","foo"]}],"extensions":{}}`, res)
}

func TestBytecodeResolveWithArgs(t *testing.T) {
	query := `query A($a: Int) {}`
	schema := TestResolveEmptyQueryDataQ{}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{}`, res)
}

func TestBytecodeResolveVariableInputWithDefault(t *testing.T) {
	query := `query A($baz: String = "foo") {bar(a: $baz)}`
	res := bytecodeParseAndExpectNoErrs(t, query, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":"foo"}`, res)
}

func TestBytecodeResolveVariable(t *testing.T) {
	query := `query A($baz: String) {bar(a: $baz)}`
	res := bytecodeParseAndExpectNoErrs(t, query, TestResolveStructTypeMethodWithPtrArgData{}, M{}, ResolveOptions{
		NoMeta:    true,
		Variables: `{"baz": "foo"}`,
	})
	Equal(t, `{"bar":"foo"}`, res)
}

type TestBytecodeResolveMultipleArgumentsData struct{}

type TestBytecodeResolveMultipleArgumentsDataIO struct {
	String string

	Int   int
	Int8  int8
	Int16 int16
	Int32 int32
	Int64 int64

	Uint   uint
	Uint8  uint8
	Uint16 uint16
	Uint32 uint32
	Uint64 uint64

	Bool bool

	Time     time.Time
	UintID   uint   `gq:"uintId,ID"`
	StringID string `gq:"stringId,ID"`

	Enum __TypeKind
}

func (TestBytecodeResolveMultipleArgumentsData) ResolveFoo(args TestBytecodeResolveMultipleArgumentsDataIO) TestBytecodeResolveMultipleArgumentsDataIO {
	return args
}

func TestBytecodeResolveMultipleArguments(t *testing.T) {
	query := `{
		foo(
			string: "abc",
			int: 123,
			int8: 123,
			int16: 123,
			int32: 123,
			int64: 123,
			uint: 123,
			uint8: 123,
			uint16: 123,
			uint32: 123,
			uint64: 123,
			bool: true,
			time: "2021-09-28T18:44:11.717Z",
			uintId: 123,
			stringId: "abc",
			enum: ENUM,
		) {
			string
			int
			int8
			int16
			int32
			int64
			uint
			uint8
			uint16
			uint32
			uint64
			bool
			time
			uintId
			stringId
			enum
		}
	}`
	schema := TestBytecodeResolveMultipleArgumentsData{}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{"foo":{"string":"abc","int":123,"int8":123,"int16":123,"int32":123,"int64":123,"uint":123,"uint8":123,"uint16":123,"uint32":123,"uint64":123,"bool":true,"time":"2021-09-28T18:44:11.717Z","uintId":"123","stringId":"abc","enum":"ENUM"}}`, res)
}

func TestBytecodeResolveMultipleArgumentsUsingDefaultVariables(t *testing.T) {
	query := `query a(
		$string: String = "abc",
		$int: Int = 123,
		$int8: Int = 123,
		$int16: Int = 123,
		$int32: Int = 123,
		$int64: Int = 123,
		$uint: Int = 123,
		$uint8: Int = 123,
		$uint16: Int = 123,
		$uint32: Int = 123,
		$uint64: Int = 123,
		$bool: Boolean = true,
		$time: Time = "2021-09-28T18:44:11.717Z",
		$uintId: ID = "123",
		$stringId: ID = "abc",
		$enum: __TypeKind = ENUM,
	) {
		foo(
			string: $string,
			int: $int,
			int8: $int8,
			int16: $int16,
			int32: $int32,
			int64: $int64,
			uint: $uint,
			uint8: $uint8,
			uint16: $uint16,
			uint32: $uint32,
			uint64: $uint64,
			bool: $bool,
			time: $time,
			uintId: $uintId,
			stringId: $stringId,
			enum: $enum,
		) {
			string
			int
			int8
			int16
			int32
			int64
			uint
			uint8
			uint16
			uint32
			uint64
			bool
			time
			uintId
			stringId
			enum
		}
	}`
	schema := TestBytecodeResolveMultipleArgumentsData{}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{"foo":{"string":"abc","int":123,"int8":123,"int16":123,"int32":123,"int64":123,"uint":123,"uint8":123,"uint16":123,"uint32":123,"uint64":123,"bool":true,"time":"2021-09-28T18:44:11.717Z","uintId":"123","stringId":"abc","enum":"ENUM"}}`, res)
}

func TestBytecodeResolveMultipleArgumentsUsingVariables(t *testing.T) {
	query := `query a(
		$string: String,
		$int: Int,
		$int8: Int,
		$int16: Int,
		$int32: Int,
		$int64: Int,
		$uint: Int,
		$uint8: Int,
		$uint16: Int,
		$uint32: Int,
		$uint64: Int,
		$bool: Boolean,
		$time: Time,
		$uintId: ID,
		$stringId: ID,
		$enum: __TypeKind,
	) {
		foo(
			string: $string,
			int: $int,
			int8: $int8,
			int16: $int16,
			int32: $int32,
			int64: $int64,
			uint: $uint,
			uint8: $uint8,
			uint16: $uint16,
			uint32: $uint32,
			uint64: $uint64,
			bool: $bool,
			time: $time,
			uintId: $uintId,
			stringId: $stringId,
			enum: $enum,
		) {
			string
			int
			int8
			int16
			int32
			int64
			uint
			uint8
			uint16
			uint32
			uint64
			bool
			time
			uintId
			stringId
			enum
		}
	}`
	schema := TestBytecodeResolveMultipleArgumentsData{}
	opts := ResolveOptions{
		NoMeta: true,
		Variables: `{
			"string": "abc",
			"int": 123,
			"int8": 123,
			"int16": 123,
			"int32": 123,
			"int64": 123,
			"uint": 123,
			"uint8": 123,
			"uint16": 123,
			"uint32": 123,
			"uint64": 123,
			"bool": true,
			"time": "2021-09-28T18:44:11.717Z",
			"uintId": "123",
			"stringId": "abc",
			"enum": "ENUM"
		}`,
	}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{}, opts)
	Equal(t, `{"foo":{"string":"abc","int":123,"int8":123,"int16":123,"int32":123,"int64":123,"uint":123,"uint8":123,"uint16":123,"uint32":123,"uint64":123,"bool":true,"time":"2021-09-28T18:44:11.717Z","uintId":"123","stringId":"abc","enum":"ENUM"}}`, res)
}

type TestBytecodeResolveJSONArrayVariableData struct{}

func (TestBytecodeResolveJSONArrayVariableData) ResolveFoo(args struct{ Data []string }) []string {
	return args.Data
}

func TestBytecodeResolveJSONArrayVariable(t *testing.T) {
	query := `query foo($data: [String]) {
		foo(data: $data)
	}`
	schema := TestBytecodeResolveJSONArrayVariableData{}
	opts := ResolveOptions{
		NoMeta: true,
		Variables: `{
			"data": ["a", "b", "c"]
		}`,
	}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{}, opts)
	Equal(t, `{"foo":["a","b","c"]}`, res)
}

type TestBytecodeResolveJSONObjectVariableData struct{}

type DataObj struct {
	A string
	C string
}

func (TestBytecodeResolveJSONObjectVariableData) ResolveFoo(args struct{ Data DataObj }) DataObj {
	return args.Data
}

func TestBytecodeResolveJSONObjectVariable(t *testing.T) {
	query := `query foo($data: DataObj__input) {
		foo(data: $data) {
			a
			c
		}
	}`
	schema := TestBytecodeResolveJSONObjectVariableData{}
	opts := ResolveOptions{
		NoMeta: true,
		Variables: `{
			"data": {"a": "b", "c": "d"}
		}`,
	}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{}, opts)
	Equal(t, `{"foo":{"a":"b","c":"d"}}`, res)
}

type TestBytecodeResolveInlineSpreadData struct {
	Inner TestBytecodeResolveInlineSpreadDataInner
}

type TestBytecodeResolveInlineSpreadDataInner struct {
	FieldA string
	FieldB string
	FieldC string
	FieldD string
}

func TestBytecodeResolveInlineSpread(t *testing.T) {
	query := `{
		inner {
			fieldA
			... on baz {
				fieldB
				fieldC
			}
			fieldD
		}
	}`
	schema := TestBytecodeResolveInlineSpreadData{
		Inner: TestBytecodeResolveInlineSpreadDataInner{
			"a",
			"b",
			"c",
			"d",
		},
	}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{"inner":{"fieldA":"a","fieldB":"b","fieldC":"c","fieldD":"d"}}`, res)
}

func TestBytecodeResolveSpread(t *testing.T) {
	query := `{
		inner {
			fieldA
			... baz
			fieldD
		}
	}

	fragment baz on Bar {
		fieldB
		fieldC
	}`

	schema := TestBytecodeResolveInlineSpreadData{
		Inner: TestBytecodeResolveInlineSpreadDataInner{
			"a",
			"b",
			"c",
			"d",
		},
	}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{"inner":{"fieldA":"a","fieldB":"b","fieldC":"c","fieldD":"d"}}`, res)
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

type TestResolveSchemaRequestSimpleData struct{}

func TestBytecodeResolveSchemaRequestSimple(t *testing.T) {
	resString := bytecodeParseAndExpectNoErrs(t, schemaQuery, TestResolveSchemaRequestSimpleData{}, M{})

	res := struct {
		Schema qlSchema `json:"__schema"`
	}{}
	err := json.Unmarshal([]byte(resString), &res)
	NoError(t, err)

	schema := res.Schema
	types := schema.JSONTypes

	Equal(t, 17, len(types))

	idx := 0
	is := func(kind, name string) {
		item := types[idx]
		Equalf(t, kind, item.JSONKind, "(KIND) Index: %d", idx)
		NotNilf(t, item.Name, "(NAME) Index: %d", idx)
		Equalf(t, name, *item.Name, "(NAME) Index: %d", idx)
		idx++
	}

	is("SCALAR", "Boolean")
	is("SCALAR", "File")
	is("SCALAR", "Float")
	is("SCALAR", "ID")
	is("SCALAR", "Int")
	is("OBJECT", "M")
	is("SCALAR", "String")
	is("OBJECT", "TestResolveSchemaRequestSimpleData")
	is("SCALAR", "Time")
	is("OBJECT", "__Directive")
	is("ENUM", "__DirectiveLocation")
	is("OBJECT", "__EnumValue")
	is("OBJECT", "__Field")
	is("OBJECT", "__InputValue")
	is("OBJECT", "__Schema")
	is("OBJECT", "__Type")
	is("ENUM", "__TypeKind")
}

type TestResolveSchemaRequestWithFieldsData struct {
	A TestResolveSchemaRequestWithFieldsDataInnerStruct
	B struct {
		Baz string
	}
	C struct {
		FooBar []TestResolveSchemaRequestWithFieldsDataInnerStruct
	}
}

type TestResolveSchemaRequestWithFieldsDataInnerStruct struct {
	Foo *string
	Bar string
}

func (TestResolveSchemaRequestWithFieldsData) ResolveD(args struct{ Foo struct{ Bar string } }) TestResolveSchemaRequestWithFieldsDataInnerStruct {
	return TestResolveSchemaRequestWithFieldsDataInnerStruct{}
}

func TestBytecodeResolveSchemaRequestWithFields(t *testing.T) {
	resString := bytecodeParseAndExpectNoErrs(t, schemaQuery, TestResolveSchemaRequestWithFieldsData{}, M{})

	res := struct {
		Schema qlSchema `json:"__schema"`
	}{}
	err := json.Unmarshal([]byte(resString), &res)
	NoError(t, err)

	schema := res.Schema
	types := schema.JSONTypes

	Equal(t, 21, len(types))

	idx := 0
	is := func(kind, name string) int {
		item := types[idx]
		Equalf(t, kind, item.JSONKind, "(KIND) Index: %d", idx)
		NotNilf(t, item.Name, "(NAME) Index: %d", idx)
		Equalf(t, name, *item.Name, "(NAME) Index: %d", idx)
		idx++
		return idx - 1
	}

	is("SCALAR", "Boolean")
	is("SCALAR", "File")
	is("SCALAR", "Float")
	is("SCALAR", "ID")
	is("SCALAR", "Int")
	is("OBJECT", "M")
	is("SCALAR", "String")
	queryIdx := is("OBJECT", "TestResolveSchemaRequestWithFieldsData")
	is("OBJECT", "TestResolveSchemaRequestWithFieldsDataInnerStruct")
	is("SCALAR", "Time")
	is("OBJECT", "__Directive")
	is("ENUM", "__DirectiveLocation")
	is("OBJECT", "__EnumValue")
	is("OBJECT", "__Field")
	is("OBJECT", "__InputValue")
	is("OBJECT", "__Schema")
	is("OBJECT", "__Type")
	is("ENUM", "__TypeKind")
	inputIdx := is("INPUT_OBJECT", "__UnknownInput1")
	is("OBJECT", "__UnknownType1")
	is("OBJECT", "__UnknownType2")

	fields := types[queryIdx].JSONFields
	Equal(t, 6, len(fields))

	idx = 0
	isField := func(name string) {
		field := fields[idx]
		Equalf(t, name, field.Name, "(NAME) Index: %d", idx)
		if field.Name == "__type" {
			Equalf(t, "OBJECT", field.Type.JSONKind, "(KIND) Index: %d", idx)
		} else {
			Equalf(t, "NON_NULL", field.Type.JSONKind, "(KIND) Index: %d", idx)
			Equalf(t, "OBJECT", field.Type.OfType.JSONKind, "(OFTYPE KIND) Index: %d", idx)
		}
		idx++
	}

	isField("__schema")
	isField("__type")
	isField("a")
	isField("b")
	isField("c")
	isField("d")

	inFields := types[inputIdx].JSONInputFields
	Equal(t, 1, len(inFields))
}

func TestBytecodeResolveGraphqlTypenameByName(t *testing.T) {
	query := `{
		__type(name: "TestResolveSchemaRequestWithFieldsDataInnerStruct") {
			kind
			name
		}
	}`

	res := bytecodeParseAndExpectNoErrs(t, query, TestResolveSchemaRequestWithFieldsData{}, M{})
	Equal(t, `{"__type":{"kind":"OBJECT","name":"TestResolveSchemaRequestWithFieldsDataInnerStruct"}}`, res)
}

func TestBytecodeResolveGraphqlTypename(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{a {__typename}}`, TestResolveSchemaRequestWithFieldsData{}, M{})
	Equal(t, `{"a":{"__typename":"TestResolveSchemaRequestWithFieldsDataInnerStruct"}}`, res)
}

func TestBytecodeResolveTracing(t *testing.T) {
	query := `{foo{a b}}`
	schema := TestResolveStructInStructInlineData{}
	json.Unmarshal([]byte(`{"foo": {"a": "foo", "b": "bar", "c": "baz"}}`), &schema)
	opts := ResolveOptions{
		Tracing: true,
	}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{}, opts)

	parsedRes := struct {
		Extensions struct {
			Tracing tracer `json:"tracing"`
		} `json:"extensions"`
	}{}
	err := json.Unmarshal([]byte(res), &parsedRes)
	NoError(t, err)

	tracer := parsedRes.Extensions.Tracing
	Equal(t, uint8(1), tracer.Version)
	NotEqual(t, "", tracer.StartTime)
	NotEqual(t, "", tracer.EndTime)
	NotEqual(t, int64(0), tracer.Duration)

	parsing := tracer.Parsing
	NotEqual(t, int64(0), parsing.Duration)
	NotEqual(t, int64(0), parsing.StartOffset)

	validation := tracer.Validation
	Equal(t, int64(0), validation.Duration)
	NotEqual(t, int64(0), validation.StartOffset)

	for _, resolver := range tracer.Execution.Resolvers {
		NotNil(t, []byte(resolver.Path))
		NotEmpty(t, []byte(resolver.Path))
		NotEqual(t, "", resolver.ParentType)
		NotEqual(t, "", resolver.FieldName)
		NotEqual(t, "", resolver.ReturnType)
		NotEqual(t, int64(0), resolver.StartOffset)
		NotEqual(t, int64(0), resolver.Duration)
	}
}

func TestBytecodeResolveDirective(t *testing.T) {
	schema := TestResolveSimpleQueryData{A: "foo", B: "bar", C: "baz", D: "foo_bar"}

	t.Run("inside field", func(t *testing.T) {
		tests := []struct {
			name    string
			query   string
			expects string
		}{
			{
				"skip field",
				`{
					a
					b @skip(if: true)
					c
				}`,
				`{"a":"foo","c":"baz"}`,
			},
			{
				"not skip field",
				`{
					a
					b @skip(if: false)
					c
				}`,
				`{"a":"foo","b":"bar","c":"baz"}`,
			},
			{
				"do not include field",
				`{
					a
					b @include(if: false)
					c
				}`,
				`{"a":"foo","c":"baz"}`,
			},
			{
				"include field",
				`{
					a
					b @include(if: true)
					c
				}`,
				`{"a":"foo","b":"bar","c":"baz"}`,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				res := bytecodeParseAndExpectNoErrs(t, test.query, schema, M{})
				Equal(t, test.expects, res, test.query)
			})
		}
	})

	t.Run("multiple field directives", func(t *testing.T) {
		query := `{
			a
			b @foo @bar
			c
		}`

		value := 1

		s := NewSchema()
		s.RegisterDirective(Directive{
			Name:  "foo",
			Where: []DirectiveLocation{DirectiveLocationField},
			Method: func() DirectiveModifier {
				value++
				return DirectiveModifier{}
			},
		})
		s.RegisterDirective(Directive{
			Name:  "bar",
			Where: []DirectiveLocation{DirectiveLocationField},
			Method: func() DirectiveModifier {
				value++
				return DirectiveModifier{}
			},
		})

		res, errs := bytecodeParse(t, s, query, schema, M{}, ResolveOptions{
			NoMeta: true,
		})
		for _, err := range errs {
			panic(err.Error())
		}
		Equal(t, `{"a":"foo","b":"bar","c":"baz"}`, res, query)
		Equal(t, 3, value)
	})

	t.Run("inside fragment", func(t *testing.T) {
		tests := []struct {
			name    string
			query   string
			expects string
		}{
			{
				"skip inline fragment",
				`{
					a
					... on Root @skip(if: true) {
						b
					}
					c
				}`,
				`{"a":"foo","c":"baz"}`,
			},
			{
				"do not skip inline fragment",
				`{
					a
					... on Root @skip(if: false) {
						b
					}
					c
				}`,
				`{"a":"foo","b":"bar","c":"baz"}`,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				res := bytecodeParseAndExpectNoErrs(t, test.query, schema, M{})
				Equal(t, test.expects, res, test.query)
			})
		}
	})
}

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
		{uintptr_, "null"}, // We do not support this datavalue
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
		{&uintptr_, "null"}, // We do not support this datavalue
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
		c := &Ctx{schema: &Schema{Result: []byte{}}}
		v := reflect.ValueOf(option.value)
		c.valueToJson(v, v.Kind())
		Equal(t, option.expect, string(c.schema.Result))
	}
}

type TestResolveWithFileData struct{}

func (TestResolveWithFileData) ResolveFoo(args struct{ File *multipart.FileHeader }) string {
	if args.File == nil {
		return ""
	}
	f, err := args.File.Open()
	if err != nil {
		return ""
	}
	defer f.Close()
	fileContents, err := ioutil.ReadAll(f)
	if err != nil {
		return ""
	}
	return string(fileContents)
}

func TestResolveBytecodeWithFile(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	multiPartWriter := multipart.NewWriter(buf)
	writer, err := multiPartWriter.CreateFormFile("FILE_ID", "test.txt")
	if err != nil {
		panic(err)
	}
	writer.Write([]byte("hello world"))
	boundary := multiPartWriter.Boundary()
	err = multiPartWriter.Close()
	if err != nil {
		panic(err)
	}

	multiPartReader := multipart.NewReader(buf, boundary)
	form, err := multiPartReader.ReadForm(1024 * 1024)
	if err != nil {
		panic(err)
	}

	opts := ResolveOptions{
		NoMeta: true,
		GetFormFile: func(key string) (*multipart.FileHeader, error) {
			f, ok := form.File[key]
			if !ok || len(f) == 0 {
				return nil, nil
			}
			return f[0], nil
		},
	}

	out, errs := bytecodeParse(t, NewSchema(), `{foo(file: "FILE_ID")}`, TestResolveWithFileData{}, M{}, opts)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"foo":"hello world"}`, out)
}

type TestResolveMaxDeptData struct {
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
	s := NewSchema()
	s.MaxDepth = 3
	out, errs := bytecodeParse(t, s, `{foo{bar{baz{fooBar{barBaz{bazFoo}}}}}}`, TestResolveMaxDeptData{}, M{}, ResolveOptions{})
	Greater(t, len(errs), 0)
	Equal(t, `{"data":{"foo":{"bar":{"baz":null}}},"errors":[{"message":"reached max dept","path":["foo","bar","baz"]}],"extensions":{}}`, out)
}

type TestResolveStructTypeMethodWithCtxData struct{}

func (TestResolveStructTypeMethodWithCtxData) ResolveBar(c *Ctx) TestResolveStructTypeMethodWithCtxDataInner {
	c.SetValue("baz", "bar")
	return TestResolveStructTypeMethodWithCtxDataInner{}
}

type TestResolveStructTypeMethodWithCtxDataInner struct{}

func (TestResolveStructTypeMethodWithCtxDataInner) ResolveFoo(c *Ctx) string {
	return c.GetValue("baz").(string)
}

func (TestResolveStructTypeMethodWithCtxData) ResolveBaz(c *Ctx) (string, error) {
	value, ok := c.GetValueOk("baz")
	if !ok {
		return "", errors.New("baz not set by bar resolver")
	}
	return value.(string), nil
}

func TestBytecodeResolveCtxValues(t *testing.T) {
	query := `
		{
			bar {
				foo
			}
			baz
		}
	`
	res := bytecodeParseAndExpectNoErrs(t, query, TestResolveStructTypeMethodWithCtxData{}, M{})
	Equal(t, `{"bar":{"foo":"bar"},"baz":"bar"}`, res)
}

type TestPathStaysCorrectData struct {
	Bar    TestPathStaysCorrectDataBar
	Foo    []TestPathStaysCorrectDataFoo
	Baz    TestPathStaysCorrectDataBar
	FooBar []TestPathStaysCorrectDataBar
}

func (TestPathStaysCorrectData) ResolvePath(c *Ctx) string {
	return string(c.GetPath())
}

type TestPathStaysCorrectDataFoo struct {
	Bar TestPathStaysCorrectDataBar
}

func (TestPathStaysCorrectDataFoo) ResolvePath(c *Ctx) string {
	return string(c.GetPath())
}

type TestPathStaysCorrectDataBar struct {
	Foo []TestPathStaysCorrectDataFoo
}

func (TestPathStaysCorrectDataBar) ResolvePath(c *Ctx) string {
	return string(c.GetPath())
}

func TestPathStaysCorrect(t *testing.T) {
	queryType := TestPathStaysCorrectData{
		Bar: TestPathStaysCorrectDataBar{
			Foo: []TestPathStaysCorrectDataFoo{
				{Bar: TestPathStaysCorrectDataBar{}},
				{Bar: TestPathStaysCorrectDataBar{}},
			},
		},
		Foo: []TestPathStaysCorrectDataFoo{
			{Bar: TestPathStaysCorrectDataBar{}},
			{Bar: TestPathStaysCorrectDataBar{}},
		},
		Baz: TestPathStaysCorrectDataBar{
			Foo: []TestPathStaysCorrectDataFoo{
				{Bar: TestPathStaysCorrectDataBar{}},
				{Bar: TestPathStaysCorrectDataBar{}},
			},
		},
		FooBar: []TestPathStaysCorrectDataBar{
			{Foo: []TestPathStaysCorrectDataFoo{}},
			{Foo: []TestPathStaysCorrectDataFoo{}},
		},
	}

	query := `{
		path
		bar {
			path
			foo {
				path
				bar {
					path
					foo
				}
			}
		}
		foo {
			path
			bar {
				path
				foo
			}
		}
		baz {
			path
			foo {
				path
				bar {
					path
					foo
				}
			}
		}
		fooBar {
			path
			foo {
				path
				foo {
					path
					bar
				}
			}
		}
	}`

	out := bytecodeParseAndExpectNoErrs(t, query, queryType, M{})

	expectedOut := `{
		"path": "[
			\"path\"
		]",
		"bar": {
			"path": "[
				\"bar\",
				\"path\"
			]",
			"foo": [
				{
					"path": "[
						\"bar\",
						\"foo\",
						0,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"bar\",
							\"foo\",
							0,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				},
				{
					"path": "[
						\"bar\",
						\"foo\",
						1,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"bar\",
							\"foo\",
							1,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				}
			]
		},
		"foo": [
			{
				"path": "[
					\"foo\",
					0,
					\"path\"
				]",
				"bar": {
					"path": "[
						\"foo\",
						0,
						\"bar\",
						\"path\"
					]",
					"foo": null
				}
			},
			{
				"path": "[
					\"foo\",
					1,
					\"path\"
				]",
				"bar": {
					"path": "[
						\"foo\",
						1,
						\"bar\",
						\"path\"
					]",
					"foo": null
				}
			}
		],
		"baz": {
			"path": "[
				\"baz\",
				\"path\"
			]",
			"foo": [
				{
					"path": "[
						\"baz\",
						\"foo\",
						0,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"baz\",
							\"foo\",
							0,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				},
				{
					"path": "[
						\"baz\",
						\"foo\",
						1,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"baz\",
							\"foo\",
							1,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				}
			]
		},
		"fooBar": [
			{
				"path": "[
					\"fooBar\",
					0,
					\"path\"
				]",
				"foo": []
			},
			{
				"path": "[
					\"fooBar\",
					1,
					\"path\"
				]",
				"foo": []
			}
		]
	}`
	expectedOut = strings.ReplaceAll(expectedOut, " ", "")
	expectedOut = strings.ReplaceAll(expectedOut, "\n", "")
	expectedOut = strings.ReplaceAll(expectedOut, "\t", "")
	Equal(t, expectedOut, out)
}

package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"reflect"
	"strings"
	"testing"
	"time"

	a "github.com/mjarkk/go-graphql/assert"
	"github.com/mjarkk/go-graphql/helpers"
)

func bytecodeParse(t *testing.T, s *Schema, query string, queries interface{}, methods interface{}, opts ...ResolveOptions) (string, []error) {
	err := s.Parse(queries, methods, nil)
	a.NoError(t, err, query)

	// Copy so we automatically also test if all fields are copied over correctly
	s = s.Copy()

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
	a.NotEqual(t, 0, len(res), query)
	return res, errs
}

type TestResolveEmptyQueryDataQ struct{}
type M struct{}

func TestBytecodeResolveOnlyOperation(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{}`, TestResolveEmptyQueryDataQ{}, M{})
	a.Equal(t, `{}`, res)
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
	a.Equal(t, `{"a":"foo"}`, res)
}

func TestBytecodeResolveMutation(t *testing.T) {
	schema := TestResolveSimpleQueryData{
		A: "foo",
		B: "bar",
	}
	res := bytecodeParseAndExpectNoErrs(t, `mutation test {a}`, M{}, schema)
	a.Equal(t, `{"a":"foo"}`, res)

	_, errs := bytecodeParseAndExpectErrs(t, `mutation test {a}`, schema, M{})
	a.Equal(t, 1, len(errs))
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
	a.Equal(t, `{"a":"foo","b":"bar"}`, res)
}

func TestBytecodeResolveAlias(t *testing.T) {
	schema := TestResolveSimpleQueryData{
		A: "foo",
		B: "bar",
	}
	res := bytecodeParseAndExpectNoErrs(t, `{b:a}`, schema, M{})
	a.Equal(t, `{"b":"foo"}`, res)
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
			a.Equal(t, testCase.expectedResult, res)
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
	a.NoError(t, err)
	a.Equal(t, "foo", res.Foo.A)
	a.Equal(t, "bar", res.Foo.B)
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
	a.Equal(t, `{"a":{"foo":null,"bar":""},"b":{"baz":""}}`, res)
}

type TestResolveArrayData struct {
	Foo []string
}

func TestBytecodeResolveArray(t *testing.T) {
	schema := TestResolveArrayData{
		Foo: []string{"foo", "bar"},
	}
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, schema, M{})
	a.Equal(t, `{"foo":["foo","bar"]}`, res)
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
	a.Equal(t, `{"foo":[{"a":"foo","b":"bar"},{"a":"baz","b":"boz"}]}`, res)
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
	a.Equal(t, `{"t":"`+string(expect)+`"}`, res)
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
	a.Equal(t, `{"foo":"`+string(exectedOutTime)+`"}`, out)
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
	a.Equal(t, `{"foo":null,"bar":"foo","baz":"bar"}`, res)
}

type TestBytecodeResolveMethodWithErrorResData struct{}

func (TestBytecodeResolveMethodWithErrorResData) ResolveFoo() (*string, error) {
	return nil, errors.New("this is an error")
}

func TestBytecodeResolveMethodWithErrorRes(t *testing.T) {
	schema := TestBytecodeResolveMethodWithErrorResData{}
	query := `{foo}`
	res, errs := bytecodeParseAndExpectErrs(t, query, schema, M{})
	a.Equal(t, 1, len(errs))
	a.Equal(t, `this is an error`, errs[0].Error())
	a.Equal(t, `{"foo":null}`, res)
}

type TestResolveStructTypeMethodWithArgsData struct{}

func (TestResolveStructTypeMethodWithArgsData) ResolveBar(c *Ctx, args struct{ A string }) string {
	return args.A
}

func TestBytecodeResolveMethodWithArg(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar(a: "foo")}`, TestResolveStructTypeMethodWithArgsData{}, M{})
	a.Equal(t, `{"bar":"foo"}`, res)
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
	a.Equal(t, `{"foo":{"a":1,"b":2,"c":3,"d":1.1}}`, res)
}

func TestBytecodeResolveTypename(t *testing.T) {
	schema := TestResolveStructTypeMethodData{}
	res := bytecodeParseAndExpectNoErrs(t, `{__typename}`, schema, M{})
	a.Equal(t, `{"__typename":"TestResolveStructTypeMethodData"}`, res)
}

type TestResolvePtrData struct {
	Foo *string
}

type TestResolvePtrInPtrData struct {
	Foo **string
}

func TestBytecodeResolveOutputPointer(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrData{}, M{})
	a.Equal(t, `{"foo":null}`, res)

	data := "bar"
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrData{&data}, M{})
	a.Equal(t, `{"foo":"bar"}`, res)

	// Nested pointers
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrInPtrData{}, M{})
	a.Equal(t, `{"foo":null}`, res)

	ptrToData := &data
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestResolvePtrInPtrData{&ptrToData}, M{})
	a.Equal(t, `{"foo":"bar"}`, res)
}

type TestResolveStructTypeMethodWithPtrArgData struct{}

func (TestResolveStructTypeMethodWithPtrArgData) ResolveBar(c *Ctx, args struct{ A *string }) *string {
	return args.A
}

func TestBytecodeResolveMethodPointerInput(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar()}`, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	a.Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: null)}`, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	a.Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: "foo")}`, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	a.Equal(t, `{"bar":"foo"}`, res)
}

type TestBytecodeResolveMethodListInputData struct{}

func (TestBytecodeResolveMethodListInputData) ResolveBar(c *Ctx, args struct{ A []string }) []string {
	return args.A
}

func TestBytecodeResolveMethodListInput(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar()}`, TestBytecodeResolveMethodListInputData{}, M{})
	a.Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: null)}`, TestBytecodeResolveMethodListInputData{}, M{})
	a.Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: ["foo", "baz"])}`, TestBytecodeResolveMethodListInputData{}, M{})
	a.Equal(t, `{"bar":["foo","baz"]}`, res)
}

type TestResolveStructTypeMethodWithStructArgData struct{}

func (TestResolveStructTypeMethodWithStructArgData) ResolveBar(c *Ctx, args struct{ A struct{ B string } }) string {
	return args.A.B
}

func TestBytecodeResolveMethodNestedInputs(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar(a: {b: "foo"})}`, TestResolveStructTypeMethodWithStructArgData{}, M{})
	a.Equal(t, `{"bar":"foo"}`, res)
}

type TestBytecodeResolveEnumData struct {
	foo __TypeKind
}

func TestBytecodeResolveEnum(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, TestBytecodeResolveEnumData{
		foo: typeKindObject,
	}, M{})
	a.Equal(t, `{"foo":"OBJECT"}`, res)
}

type TestBytecodeResolveEnumInputData struct{}

func (TestBytecodeResolveEnumInputData) ResolveFoo(args struct{ A __TypeKind }) __TypeKind {
	return args.A
}

func TestBytecodeResolveEnumInput(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo(a: OBJECT)}`, TestBytecodeResolveEnumInputData{}, M{})
	a.Equal(t, `{"foo":"OBJECT"}`, res)
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
	a.Equal(t, `{"data":{"a":{"foo":null,"bar":""},"b":{"baz":""}}}`, res)
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
	a.Equal(t, `{"data":{"a":{"foo":null}},"errors":[{"message":"field arguments not allowed","path":["a","foo"]}],"extensions":{}}`, res)
}

func TestBytecodeResolveWithArgs(t *testing.T) {
	query := `query A($a: Int) {}`
	schema := TestResolveEmptyQueryDataQ{}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	a.Equal(t, `{}`, res)
}

func TestBytecodeResolveVariableInputWithDefault(t *testing.T) {
	query := `query A($baz: String = "foo") {bar(a: $baz)}`
	res := bytecodeParseAndExpectNoErrs(t, query, TestResolveStructTypeMethodWithPtrArgData{}, M{})
	a.Equal(t, `{"bar":"foo"}`, res)
}

func TestBytecodeResolveVariable(t *testing.T) {
	query := `query A($baz: String) {bar(a: $baz)}`
	res := bytecodeParseAndExpectNoErrs(t, query, TestResolveStructTypeMethodWithPtrArgData{}, M{}, ResolveOptions{
		NoMeta:    true,
		Variables: `{"baz": "foo"}`,
	})
	a.Equal(t, `{"bar":"foo"}`, res)
}

type TestBytecodeResolveMultipleArgumentsData struct{}

type TestBytecodeResolveMultipleArgumentsDataIO struct {
	String string `json:"string"`

	Int   int   `json:"int"`
	Int8  int8  `json:"int8"`
	Int16 int16 `json:"int16"`
	Int32 int32 `json:"int32"`
	Int64 int64 `json:"int64"`

	Uint   uint   `json:"uint"`
	Uint8  uint8  `json:"uint8"`
	Uint16 uint16 `json:"uint16"`
	Uint32 uint32 `json:"uint32"`
	Uint64 uint64 `json:"uint64"`

	Bool bool

	Time       time.Time `json:"time"`
	UintID     uint      `json:"-" gq:"uintId,ID"`
	JSONUingID string    `json:"uintID" gq:"-"` // For test output json format
	StringID   string    `json:"stringID" gq:"stringId,ID"`

	Enum     __TypeKind `json:"-"`
	JSONEnum string     `json:"enum" gq:"-"` // For test output json format

	IntPtr      *int `json:"intPtr"`
	IntPtrWData *int `json:"intPtrWData"`

	Struct struct {
		String      string `json:"string"`
		Int         int    `json:"int"`
		IntPtr      *int   `json:"intPtr"`
		IntPtrWData *int   `json:"intPtrWData"`
	} `json:"struct"`
}

func (TestBytecodeResolveMultipleArgumentsData) ResolveFoo(args TestBytecodeResolveMultipleArgumentsDataIO) TestBytecodeResolveMultipleArgumentsDataIO {
	return args
}

func TestBytecodeResolveMultipleArguments(t *testing.T) {
	outFields := `{
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
		intPtr
		intPtrWData
		struct {
			string
			int
			intPtr
			intPtrWData
		}
	}`
	schema := TestBytecodeResolveMultipleArgumentsData{}

	testCases := []struct {
		Name string
		Exec func(t *testing.T) string
	}{
		{
			"direct arguments",
			func(t *testing.T) string {
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
						intPtr: null,
						intPtrWData: 123,
						struct: {
							string: "abc",
							int: 123,
							intPtr: null,
							intPtrWData: 123,
						},
					) ` + outFields + `
				}`
				return bytecodeParseAndExpectNoErrs(t, query, schema, M{})
			},
		},
		{
			"variables with default value",
			func(t *testing.T) string {
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
					$intPtr: Int = null,
					$intPtrWData: Int = 123,
					$struct: __UnknownInput1 = {
						string: "abc",
						int: 123,
						intPtr: null,
						intPtrWData: 123,
					},
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
						intPtr: $intPtr,
						intPtrWData: $intPtrWData,
						struct: $struct,
					) ` + outFields + `
				}`
				return bytecodeParseAndExpectNoErrs(t, query, schema, M{})
			},
		},
		{
			"variables",
			func(t *testing.T) string {
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
					$intPtr: Int,
					$intPtrWData: Int,
					$struct: __UnknownInput1,
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
						intPtr: $intPtr,
						intPtrWData: $intPtrWData,
						struct: $struct,
					) ` + outFields + `
				}`
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
						"enum": "ENUM",
						"intPtr": null,
						"intPtrWData": 123,
						"struct":{
							"string": "abc",
							"int": 123,
							"intPtr": null,
							"intPtrWData": 123
						}
					}`,
				}
				return bytecodeParseAndExpectNoErrs(t, query, schema, M{}, opts)
			},
		},
	}

	formatJSON := func(t *testing.T, in string) string {
		inParsed := struct {
			Foo TestBytecodeResolveMultipleArgumentsDataIO `json:"foo"`
		}{}
		err := json.Unmarshal([]byte(in), &inParsed)
		a.NoError(t, err)
		out, err := json.MarshalIndent(inParsed, "", "  ")
		a.NoError(t, err)
		return string(out)
	}

	expected := `{
		"foo":{
			"string":"abc",
			"int":123,
			"int8":123,
			"int16":123,
			"int32":123,
			"int64":123,
			"uint":123,
			"uint8":123,
			"uint16":123,
			"uint32":123,
			"uint64":123,
			"bool":true,
			"time":"2021-09-28T18:44:11.717Z",
			"uintId":"123",
			"stringId":"abc",
			"enum":"ENUM",
			"intPtr":null,
			"intPtrWData":123,
			"struct":{
				"string":"abc",
				"int": 123,
				"intPtr":null,
				"intPtrWData":123
			}
		}
	}`
	formattedExpected := formatJSON(t, expected)

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			resJSON := testCase.Exec(t)
			formattedResJSON := formatJSON(t, resJSON)
			a.Equal(t, formattedExpected, formattedResJSON)
		})
	}
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
	a.Equal(t, `{"foo":["a","b","c"]}`, res)
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
	a.Equal(t, `{"foo":{"a":"b","c":"d"}}`, res)
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
			... on TestBytecodeResolveInlineSpreadDataInner {
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
	a.Equal(t, `{"inner":{"fieldA":"a","fieldB":"b","fieldC":"c","fieldD":"d"}}`, res)
}

func TestBytecodeResolveSpread(t *testing.T) {
	query := `{
		inner {
			fieldA
			... baz
			fieldD
		}
	}

	fragment baz on TestBytecodeResolveInlineSpreadDataInner {
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
	a.Equal(t, `{"inner":{"fieldA":"a","fieldB":"b","fieldC":"c","fieldD":"d"}}`, res)
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
	a.NoError(t, err)

	schema := res.Schema
	types := schema.JSONTypes

	a.Equal(t, 17, len(types))

	idx := 0
	is := func(kind, name string) {
		item := types[idx]
		a.Equalf(t, kind, item.JSONKind, "(KIND) Index: %d", idx)
		a.NotNilf(t, item.Name, "(NAME) Index: %d", idx)
		a.Equalf(t, name, *item.Name, "(NAME) Index: %d", idx)
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

type DArgs struct {
	InnerStruct TestBytecodeResolveMultipleArgumentsDataIO
	Arr         []string
	Ptr         *string
	Float       float64
	File        *multipart.FileHeader
}

func (TestResolveSchemaRequestWithFieldsData) ResolveD(args DArgs) TestResolveSchemaRequestWithFieldsDataInnerStruct {
	return TestResolveSchemaRequestWithFieldsDataInnerStruct{}
}

func TestBytecodeResolveSchemaRequestWithFields(t *testing.T) {
	resString := bytecodeParseAndExpectNoErrs(t, schemaQuery, TestResolveSchemaRequestWithFieldsData{}, M{})

	res := struct {
		Schema qlSchema `json:"__schema"`
	}{}
	err := json.Unmarshal([]byte(resString), &res)
	a.NoError(t, err)

	schema := res.Schema
	types := schema.JSONTypes

	a.Equal(t, 22, len(types))

	idx := 0
	is := func(kind, name string) int {
		item := types[idx]
		a.Equalf(t, kind, item.JSONKind, "(KIND) Index: %d, name: %s", idx, name)
		a.NotNilf(t, item.Name, "(NAME) Index: %d", idx)
		a.Equalf(t, name, *item.Name, "(NAME) Index: %d", idx)
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
	inputIdx := is("INPUT_OBJECT", "TestBytecodeResolveMultipleArgumentsDataIO")
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
	is("INPUT_OBJECT", "__UnknownInput1")
	is("OBJECT", "__UnknownType1")
	is("OBJECT", "__UnknownType2")

	fields := types[queryIdx].JSONFields
	a.Equal(t, 4, len(fields))

	idx = 0
	isField := func(name string) {
		field := fields[idx]
		a.Equalf(t, name, field.Name, "(NAME) Index: %d", idx)
		if field.Name == "__type" {
			a.Equalf(t, "OBJECT", field.Type.JSONKind, "(KIND) Index: %d", idx)
		} else {
			a.Equalf(t, "NON_NULL", field.Type.JSONKind, "(KIND) Index: %d", idx)
			a.Equalf(t, "OBJECT", field.Type.OfType.JSONKind, "(OFTYPE KIND) Index: %d", idx)
		}
		idx++
	}

	isField("a")
	isField("b")
	isField("c")
	isField("d")

	inFields := types[inputIdx].JSONInputFields
	a.Equal(t, 19, len(inFields))
}

func TestBytecodeResolveGraphqlTypenameByName(t *testing.T) {
	query := `{
		__type(name: "TestResolveSchemaRequestWithFieldsDataInnerStruct") {
			kind
			name
		}
	}`

	res := bytecodeParseAndExpectNoErrs(t, query, TestResolveSchemaRequestWithFieldsData{}, M{})
	a.Equal(t, `{"__type":{"kind":"OBJECT","name":"TestResolveSchemaRequestWithFieldsDataInnerStruct"}}`, res)
}

func TestBytecodeResolveGraphqlTypename(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{a {__typename}}`, TestResolveSchemaRequestWithFieldsData{}, M{})
	a.Equal(t, `{"a":{"__typename":"TestResolveSchemaRequestWithFieldsDataInnerStruct"}}`, res)
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
	a.NoError(t, err)

	tracer := parsedRes.Extensions.Tracing
	a.Equal(t, uint8(1), tracer.Version)
	a.NotEqual(t, "", tracer.StartTime)
	a.NotEqual(t, "", tracer.EndTime)
	a.NotEqual(t, int64(0), tracer.Duration)

	parsing := tracer.Parsing
	a.NotEqual(t, int64(0), parsing.Duration)

	validation := tracer.Validation
	a.Equal(t, int64(0), validation.Duration)
	a.NotEqual(t, int64(0), validation.StartOffset)

	for _, resolver := range tracer.Execution.Resolvers {
		a.NotNil(t, []byte(resolver.Path))
		a.NotEmpty(t, []byte(resolver.Path))
		a.NotEqual(t, "", resolver.ParentType)
		a.NotEqual(t, "", resolver.FieldName)
		a.NotEqual(t, "", resolver.ReturnType)
		a.NotEqual(t, int64(0), resolver.StartOffset)
		a.NotEqual(t, int64(0), resolver.Duration)
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
				a.Equal(t, test.expects, res, test.query)
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
		a.Equal(t, `{"a":"foo","b":"bar","c":"baz"}`, res, query)
		a.Equal(t, 3, value)
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
					... on TestResolveSimpleQueryData @skip(if: true) {
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
					... on TestResolveSimpleQueryData @skip(if: false) {
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
				a.Equal(t, test.expects, res, test.query)
			})
		}
	})
}

func TestValueToJson(t *testing.T) {
	stringValue := string(`a"b`)
	boolTrue := bool(true)
	boolFalse := bool(false)
	intValue := int(1)
	int8Value := int8(2)
	int16Value := int16(3)
	int32Value := int32(4)
	int64Value := int64(5)
	uintValue := uint(6)
	uint8Value := uint8(7)
	uint16Value := uint16(8)
	uint32Value := uint32(9)
	uint64Value := uint64(10)
	uintptrValue := uintptr(11)
	float32Value := float32(12)
	float64Value := float64(13)
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
		{stringValue, `"a\"b"`},
		{boolTrue, "true"},
		{boolFalse, "false"},
		{intValue, "1"},
		{int8Value, "2"},
		{int16Value, "3"},
		{int32Value, "4"},
		{int64Value, "5"},
		{uintValue, "6"},
		{uint8Value, "7"},
		{uint16Value, "8"},
		{uint32Value, "9"},
		{uint64Value, "10"},
		{uintptrValue, "null"}, // We do not support this datavalue
		{float32Value, "12"},
		{float64Value, "13"},
		{float64WExponent, "1e-98"},

		{&stringValue, `"a\"b"`},
		{&boolTrue, "true"},
		{&boolFalse, "false"},
		{&intValue, "1"},
		{&int8Value, "2"},
		{&int16Value, "3"},
		{&int32Value, "4"},
		{&int64Value, "5"},
		{&uintValue, "6"},
		{&uint8Value, "7"},
		{&uint16Value, "8"},
		{&uint32Value, "9"},
		{&uint64Value, "10"},
		{&uintptrValue, "null"}, // We do not support this datavalue
		{&float32Value, "12"},
		{&float64Value, "13"},

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
		c.valueToJSON(v, v.Kind())
		a.Equal(t, option.expect, string(c.schema.Result))
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
	a.Equal(t, `{"foo":"hello world"}`, out)
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
	a.Greater(t, len(errs), 0)
	a.Equal(t, `{"data":{"foo":{"bar":{"baz":null}}},"errors":[{"message":"reached max dept","path":["foo","bar","baz"]}],"extensions":{}}`, out)
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
	a.Equal(t, `{"bar":{"foo":"bar"},"baz":"bar"}`, res)
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
	expectedOut = strings.NewReplacer(
		" ", "",
		"\n", "",
		"\t", "",
	).Replace(expectedOut)
	a.Equal(t, expectedOut, out)
}

type NotRegisteredInterfaceType struct{}

func (NotRegisteredInterfaceType) ResolveFoo() string { return "THIS SHOULD NOT APPEAR IN RESULTS" }
func (NotRegisteredInterfaceType) ResolveBar() string { return "THIS SHOULD NOT APPEAR IN RESULTS" }

func TestBytecodeResolveInterface(t *testing.T) {
	Implements((*InterfaceType)(nil), BarWImpl{})
	Implements((*InterfaceType)(nil), BazWImpl{})

	testCases := []struct {
		name           string
		interfaceValue InterfaceType
		expect         string
	}{
		{
			"nil interface value",
			nil,
			"null",
		},
		{
			"struct 1 implementing interface",
			BarWImpl{},
			`{"foo":"this is bar","bar":"This is bar"}`,
		},
		{
			"struct 2 implementing interface",
			BazWImpl{},
			`{"foo":"this is baz","bar":"This is baz"}`,
		},
		{
			"struct that implements interface but is not registered",
			NotRegisteredInterfaceType{},
			`null`,
		},
	}

	query := `{
		generic {foo bar}
	}`

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			querySchema := InterfaceSchema{
				Bar:     BarWImpl{},
				Baz:     BazWImpl{},
				Generic: testCase.interfaceValue,
			}

			out := bytecodeParseAndExpectNoErrs(t, query, querySchema, M{})
			a.Equal(t, `{"generic":`+testCase.expect+`}`, out)
		})
	}

}

type TestBytecodeResolveInterfaceArrayData struct {
	TheList []InterfaceType
}

func TestBytecodeResolveInterfaceArray(t *testing.T) {
	Implements((*InterfaceType)(nil), BarWImpl{})
	Implements((*InterfaceType)(nil), BazWImpl{})

	querySchema := TestBytecodeResolveInterfaceArrayData{
		TheList: []InterfaceType{
			BarWImpl{},
			BazWImpl{},
			nil,
		},
	}
	query := `{theList{foo bar}}`

	out := bytecodeParseAndExpectNoErrs(t, query, querySchema, M{})
	a.Equal(t, `{"theList":[{"foo":"this is bar","bar":"This is bar"},{"foo":"this is baz","bar":"This is baz"},null]}`, out)
}

func TestBytecodeResolveInterfaceType(t *testing.T) {
	Implements((*InterfaceType)(nil), BarWImpl{})
	Implements((*InterfaceType)(nil), BazWImpl{})

	query := `{
		__type(name: "InterfaceType") {
			name
			fields(includeDeprecated: true) {name}
			possibleTypes {kind name}
		}
	}`

	out := bytecodeParseAndExpectNoErrs(t, query, InterfaceSchema{}, M{})
	a.Equal(t, `{"__type":{"name":"InterfaceType","fields":[{"name":"bar"},{"name":"foo"}],"possibleTypes":[{"kind":"OBJECT","name":"BarWImpl"},{"kind":"OBJECT","name":"BazWImpl"}]}}`, out)
}

func TestBytecodeResolveInterfaceArrayWithFragment(t *testing.T) {
	Implements((*InterfaceType)(nil), BarWImpl{})
	Implements((*InterfaceType)(nil), BazWImpl{})

	querySchema := TestBytecodeResolveInterfaceArrayData{
		TheList: []InterfaceType{
			BarWImpl{ExtraBarField: "bar"},
			BazWImpl{ExtraBazField: "baz"},
			nil,
		},
	}
	query := `{
		theList{
			foo
			bar
			... on BarWImpl { extraBarField }
			... on BazWImpl { extraBazField }
		}
	}`

	out := bytecodeParseAndExpectNoErrs(t, query, querySchema, M{})
	a.Equal(t, `{"theList":[{"foo":"this is bar","bar":"This is bar","extraBarField":"bar"},{"foo":"this is baz","bar":"This is baz","extraBazField":"baz"},null]}`, out)
}

type TestBytecodeResolveContextData struct{}

func (TestBytecodeResolveContextData) ResolveFoo(ctx *Ctx) bool {
	<-ctx.context.Done()
	return true
}

func TestBytecodeResolveContext(t *testing.T) {
	context, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	opts := ResolveOptions{NoMeta: true, Context: context}
	out, errs := bytecodeParseAndExpectErrs(t, `{foo}`, TestBytecodeResolveContextData{}, M{}, opts)
	a.Equal(t, 1, len(errs))
	a.Equal(t, `{"foo":null}`, out)
}

func TestBytecodeResolveQueryCache(t *testing.T) {
	testCases := []struct {
		query  string
		result string
	}{
		{`{a}`, `{"a":"1"}`},
		{`{b}`, `{"b":"2"}`},
		{`{c}`, `{"c":"3"}`},
		{`{d}`, `{"d":"4"}`},
		{`{a,b}`, `{"a":"1","b":"2"}`},
		{`{b,c}`, `{"b":"2","c":"3"}`},
		{`{c,d}`, `{"c":"3","d":"4"}`},
		{`{a,b,c}`, `{"a":"1","b":"2","c":"3"}`},
		{`{b,c,d}`, `{"b":"2","c":"3","d":"4"}`},
	}

	s := NewSchema()

	err := s.Parse(TestResolveSimpleQueryData{
		A: "1",
		B: "2",
		C: "3",
		D: "4",
	}, M{}, nil)
	a.NoError(t, err)

	cacheQueryFromLen := 0
	s.SetCacheRules(&cacheQueryFromLen)

	for i := 0; i < 20; i++ {
		for padding := 0; padding < 100; padding++ {
			for _, testCase := range testCases {
				query := testCase.query + strings.Repeat(" ", padding)
				errs := s.Resolve([]byte(query), ResolveOptions{NoMeta: true})
				for _, err := range errs {
					panic(err)
				}
				a.Equal(t, testCase.result, string(s.Result), query, i)
			}
		}
	}
}

type TestBytecodeResolveIDData struct {
	DirectID int                    `gq:"directId,id"`
	MethodID func() (int, AttrIsID) `gq:"methodId"`
}

func TestBytecodeResolveID(t *testing.T) {
	schema := TestBytecodeResolveIDData{
		DirectID: 2,
		MethodID: func() (int, AttrIsID) {
			return 3, 0
		},
	}
	query := `{directId,methodId}`
	out := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	a.Equal(t, `{"directId":"2","methodId":"3"}`, out)
}

package graphql

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/stretchr/testify/assert"
)

func bytecodeParse(t *testing.T, query string, queries interface{}, methods interface{}, opts ...BytecodeParseOptions) (string, []error) {
	s, err := ParseSchema(queries, methods, nil)
	NoError(t, err, query)

	ctx := NewBytecodeCtx(s)
	if len(opts) == 0 {
		opts = []BytecodeParseOptions{{NoMeta: true}}
	}
	errs := ctx.BytecodeResolve([]byte(query), opts[0])
	return string(ctx.Result), errs
}

func bytecodeParseAndExpectNoErrs(t *testing.T, query string, queries interface{}, methods interface{}, opts ...BytecodeParseOptions) string {
	res, errs := bytecodeParse(t, query, queries, methods, opts...)
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
	schema := TestExecSimpleQueryData{
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
	schema := TestExecSimpleQueryData{
		A: "foo",
		B: "bar",
	}
	res := bytecodeParseAndExpectNoErrs(t, `{b:a}`, schema, M{})
	Equal(t, `{"b":"foo"}`, res)
}

func TestBytecodeResolveOperatorWithName(t *testing.T) {
	schema := TestExecSimpleQueryData{
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
			res, errs := bytecodeParse(t, `query a {a} query b {b}`, schema, M{}, BytecodeParseOptions{
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

func TestBytecodeResolveMultipleNestedFields(t *testing.T) {
	schema := TestExecSchemaRequestWithFieldsData{}
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

func TestBytecodeResolveArray(t *testing.T) {
	schema := TestExecArrayData{
		Foo: []string{"foo", "bar"},
	}
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, schema, M{})
	Equal(t, `{"foo":["foo","bar"]}`, res)
}

type TestBytecodeResolveStructsArrayData struct {
	Foo []TestExecSimpleQueryData
}

func TestBytecodeResolveStructsArray(t *testing.T) {
	schema := TestBytecodeResolveStructsArrayData{
		Foo: []TestExecSimpleQueryData{
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
	expect := now.Format(timeISO8601Layout)

	schema := TestBytecodeResolveTimeData{now}
	res := bytecodeParseAndExpectNoErrs(t, `{t}`, schema, M{})
	Equal(t, `{"t":"`+expect+`"}`, res)
}

func TestBytecodeResolveTimeIO(t *testing.T) {
	now := time.Now()
	testTimeInput := []byte{}
	timeToString(&testTimeInput, now)

	query := `{foo(t: "` + string(testTimeInput) + `")}`
	out := bytecodeParseAndExpectNoErrs(t, query, TestExecTimeIOData{}, M{})

	exectedOutTime := []byte{}
	timeToString(&exectedOutTime, now.AddDate(3, 2, 1).Add(time.Hour+time.Second))
	Equal(t, `{"foo":"`+string(exectedOutTime)+`"}`, out)
}

func TestBytecodeResolveMethod(t *testing.T) {
	schema := TestExecStructTypeMethodData{}
	res := bytecodeParseAndExpectNoErrs(t, `{foo, bar, baz}`, schema, M{})
	Equal(t, `{"foo":null,"bar":"foo","baz":"bar"}`, res)
}

func TestBytecodeResolveMethodWithArg(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar(a: "foo")}`, TestExecStructTypeMethodWithArgsData{}, M{})
	Equal(t, `{"bar":"foo"}`, res)
}

func TestBytecodeResolveMethodWithIntArgs(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo(a: 1, b: 2, c: 3, d: 1.1) {a b c d}}`, TestExecInputAllKindsOfNumbersData{}, M{})
	Equal(t, `{"foo":{"a":1,"b":2,"c":3,"d":1.1}}`, res)
}

func TestBytecodeResolveTypename(t *testing.T) {
	schema := TestExecStructTypeMethodData{}
	res := bytecodeParseAndExpectNoErrs(t, `{__typename}`, schema, M{})
	Equal(t, `{"__typename":"TestExecStructTypeMethodData"}`, res)
}

func TestBytecodeResolveOutputPointer(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{foo}`, TestExecPtrData{}, M{})
	Equal(t, `{"foo":null}`, res)

	data := "bar"
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestExecPtrData{&data}, M{})
	Equal(t, `{"foo":"bar"}`, res)

	// Nested pointers
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestExecPtrInPtrData{}, M{})
	Equal(t, `{"foo":null}`, res)

	ptrToData := &data
	res = bytecodeParseAndExpectNoErrs(t, `{foo}`, TestExecPtrInPtrData{&ptrToData}, M{})
	Equal(t, `{"foo":"bar"}`, res)
}

func TestBytecodeResolveMethodPointerInput(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar()}`, TestExecStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: null)}`, TestExecStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":null}`, res)

	res = bytecodeParseAndExpectNoErrs(t, `{bar(a: "foo")}`, TestExecStructTypeMethodWithPtrArgData{}, M{})
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

func TestBytecodeResolveMethodNestedInputs(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{bar(a: {b: "foo"})}`, TestExecStructTypeMethodWithStructArgData{}, M{})
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
	schema := TestExecSchemaRequestWithFieldsData{}
	res, _ := bytecodeParse(t, query, schema, M{}, BytecodeParseOptions{})
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
	schema := TestExecSchemaRequestWithFieldsData{}
	res, _ := bytecodeParse(t, query, schema, M{}, BytecodeParseOptions{})
	if !json.Valid([]byte(res)) {
		panic("invalid json: " + res)
	}
	Equal(t, `{"data":{"a":{"foo":null}},"errors":[{"message":"field arguments not allowed","path":["a","foo"]}],"extensions":{}}`, res)
}

func TestBytecodeResolveWithArgs(t *testing.T) {
	query := `query A($a: Int) {}`
	schema := TestExecEmptyQueryDataQ{}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{}`, res)
}

func TestBytecodeResolveVariableInputWithDefault(t *testing.T) {
	query := `query A($baz: String = "foo") {bar(a: $baz)}`
	res := bytecodeParseAndExpectNoErrs(t, query, TestExecStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":"foo"}`, res)
}

func TestBytecodeResolveVariable(t *testing.T) {
	query := `query A($baz: String) {bar(a: $baz)}`
	res := bytecodeParseAndExpectNoErrs(t, query, TestExecStructTypeMethodWithPtrArgData{}, M{}, BytecodeParseOptions{
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

	Time time.Time
	ID   uint `gq:"id,ID"`
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
			id: 123,
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
			id
		}
	}`
	schema := TestBytecodeResolveMultipleArgumentsData{}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{"foo":{"string":"abc","int":123,"int8":123,"int16":123,"int32":123,"int64":123,"uint":123,"uint8":123,"uint16":123,"uint32":123,"uint64":123,"bool":true,"time":"2021-09-28T18:44:11.717Z","id":"123"}}`, res)
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
		$id: ID = "123",
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
			id: $id,
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
			id
		}
	}`
	schema := TestBytecodeResolveMultipleArgumentsData{}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{})
	Equal(t, `{"foo":{"string":"abc","int":123,"int8":123,"int16":123,"int32":123,"int64":123,"uint":123,"uint8":123,"uint16":123,"uint32":123,"uint64":123,"bool":true,"time":"2021-09-28T18:44:11.717Z","id":"123"}}`, res)
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
		$id: ID,
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
			id: $id,
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
			id
		}
	}`
	schema := TestBytecodeResolveMultipleArgumentsData{}
	opts := BytecodeParseOptions{
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
			"id": "123"
		}`,
	}
	res := bytecodeParseAndExpectNoErrs(t, query, schema, M{}, opts)
	Equal(t, `{"foo":{"string":"abc","int":123,"int8":123,"int16":123,"int32":123,"int64":123,"uint":123,"uint8":123,"uint16":123,"uint32":123,"uint64":123,"bool":true,"time":"2021-09-28T18:44:11.717Z","id":"123"}}`, res)
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
	opts := BytecodeParseOptions{
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
	opts := BytecodeParseOptions{
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

func TestBytecodeResolveSchemaRequestSimple(t *testing.T) {
	resString := bytecodeParseAndExpectNoErrs(t, schemaQuery, TestExecSchemaRequestSimpleData{}, M{})

	res := struct {
		Schema qlSchema `json:"__schema"`
	}{}
	err := json.Unmarshal([]byte(resString), &res)
	NoError(t, err)

	schema := res.Schema
	types := schema.JSONTypes

	totalTypes := 17
	if testingRegisteredTestEnum {
		totalTypes++
	}
	Equal(t, totalTypes, len(types))

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
	if testingRegisteredTestEnum {
		is("ENUM", "TestEnum2")
	}
	is("OBJECT", "TestExecSchemaRequestSimpleData")
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

func TestBytecodeResolveSchemaRequestWithFields(t *testing.T) {
	resString := bytecodeParseAndExpectNoErrs(t, schemaQuery, TestExecSchemaRequestWithFieldsData{}, M{})

	res := struct {
		Schema qlSchema `json:"__schema"`
	}{}
	err := json.Unmarshal([]byte(resString), &res)
	NoError(t, err)

	schema := res.Schema
	types := schema.JSONTypes

	totalTypes := 21
	if testingRegisteredTestEnum {
		totalTypes++
	}
	Equal(t, totalTypes, len(types))

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
	if testingRegisteredTestEnum {
		is("ENUM", "TestEnum2")
	}
	queryIdx := is("OBJECT", "TestExecSchemaRequestWithFieldsData")
	is("OBJECT", "TestExecSchemaRequestWithFieldsDataInnerStruct")
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
		__type(name: "TestExecSchemaRequestWithFieldsDataInnerStruct") {
			kind
			name
		}
	}`

	res := bytecodeParseAndExpectNoErrs(t, query, TestExecSchemaRequestWithFieldsData{}, M{})
	Equal(t, `{"__type":{"kind":"OBJECT","name":"TestExecSchemaRequestWithFieldsDataInnerStruct"}}`, res)
}

func TestBytecodeResolveGraphqlTypename(t *testing.T) {
	res := bytecodeParseAndExpectNoErrs(t, `{a {__typename}}`, TestExecSchemaRequestWithFieldsData{}, M{})
	Equal(t, `{"a":{"__typename":"TestExecSchemaRequestWithFieldsDataInnerStruct"}}`, res)
}

func TestBytecodeResolveTracing(t *testing.T) {
	query := `{foo{a b}}`
	schema := TestExecStructInStructInlineData{}
	json.Unmarshal([]byte(`{"foo": {"a": "foo", "b": "bar", "c": "baz"}}`), &schema)
	opts := BytecodeParseOptions{
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
	LessOrEqual(t, parsing.StartOffset+parsing.Duration, validation.StartOffset)

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

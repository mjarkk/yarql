package graphql

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/mjarkk/go-graphql/bytecode"
	. "github.com/stretchr/testify/assert"
)

func bytecodeParseWithOpts(t *testing.T, query string, queries interface{}, methods interface{}, opts BytecodeParseOptions) (string, []error) {
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
	bytes, errs := ctx.BytecodeResolve([]byte(query), opts)
	return string(bytes), errs
}

func bytecodeParse(t *testing.T, query string, queries interface{}, methods interface{}) (string, []error) {
	return bytecodeParseWithOpts(t, query, queries, methods, BytecodeParseOptions{NoMeta: true})
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
			res, errs := bytecodeParseWithOpts(t, `query a {a} query b {b}`, schema, M{}, BytecodeParseOptions{
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
	res, _ := bytecodeParseWithOpts(t, query, schema, M{}, BytecodeParseOptions{})
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
	res, _ := bytecodeParseWithOpts(t, query, schema, M{}, BytecodeParseOptions{})
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

func TestBytecodeVariableInputWithDefault(t *testing.T) {
	query := `query A($baz: String = "foo") {bar(a: $baz)}`
	res := bytecodeParseAndExpectNoErrs(t, query, TestExecStructTypeMethodWithPtrArgData{}, M{})
	Equal(t, `{"bar":"foo"}`, res)
}

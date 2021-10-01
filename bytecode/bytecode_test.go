package bytecode

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func parseQuery(query string) ([]byte, []error) {
	i := ParserCtx{
		Res:               []byte{},
		FragmentLocations: []int{},
		Query:             []byte(query),
		Errors:            []error{},
		Hasher:            fnv.New32(),
	}
	i.ParseQueryToBytecode(nil)
	return i.Res, i.Errors
}

func parseQueryAndExpectErr(t *testing.T, query, expectedErr string) {
	_, errs := parseQuery(query)
	if len(errs) == 0 {
		Fail(t, "exected query to fail with error: "+expectedErr, query)
	}
	Equal(t, errs[0].Error(), expectedErr)
}

func newParseQueryAndExpectResult(t *testing.T, query string, expectedResult []byte, debug ...bool) {
	res, errs := parseQuery(query)
	for _, err := range errs {
		panic(err.Error())
	}

	resHex := hex.Dump(res)
	expectedResultHex := hex.Dump(expectedResult)

	if len(debug) > 0 && debug[0] {
		fmt.Println("OUT")
		fmt.Println(resHex)

		fmt.Println("EXPECTED RESULT")
		fmt.Println(expectedResultHex)
	}

	Equal(t, expectedResultHex, resHex, query)
}

func TestParseSimpleQuery(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{}`,
		testOperator{}.toBytes(),
	)
}

func TestParseSimpleQueryWrittenOut(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`query {}`,
		testOperator{}.toBytes(),
	)
}

func TestParseSimpleMutation(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`mutation {}`,
		testOperator{
			kind: OperatorMutation,
		}.toBytes(),
	)
}

func TestParseSimpleSubscription(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`subscription {}`,
		testOperator{
			kind: OperatorSubscription,
		}.toBytes(),
	)
}

func TestParseQueryWithName(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`query banana {}`,
		testOperator{
			name: "banana",
		}.toBytes(),
	)
}

func TestParseQuerywithArgs(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`query banana($quality: [Int]) {}`,
		testOperator{
			name: "banana",
			args: []testOperatorArg{
				{name: "quality", type_: "lnInt"},
			},
		}.toBytes(),
	)

	newParseQueryAndExpectResult(
		t,
		`query banana($quality: [Int!]! = [10]) {}`,
		testOperator{
			name: "banana",
			args: []testOperatorArg{
				{
					name:  "quality",
					type_: "LNInt",
					defaultValue: &testValue{
						kind: ValueList,
						list: []testValue{
							{kind: ValueInt, intValue: 10},
						},
					},
				},
			},
		}.toBytes(),
	)

	newParseQueryAndExpectResult(
		t,
		`query foo($bar: String = "bar", $baz: String = "baz") {}`,
		testOperator{
			name: "foo",
			args: []testOperatorArg{
				{
					name:         "bar",
					type_:        "nString",
					defaultValue: &testValue{kind: ValueString, stringValue: "bar"},
				},
				{
					name:         "baz",
					type_:        "nString",
					defaultValue: &testValue{kind: ValueString, stringValue: "baz"},
				},
			},
		}.toBytes(),
	)

	injectCodeSurviveTest(`query banana($quality: [Int!]! = [10]) {}`)
}

func TestParseMultipleSimpleQueries(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{}{}`,
		append(
			testOperator{}.toBytes(),
			testOperator{}.toBytes()...,
		),
	)
}

func TestParseMultipleQueries(t *testing.T) {
	query := `
		query a {}
		mutation b {}
	`

	newParseQueryAndExpectResult(
		t,
		query,
		append(
			testOperator{name: "a", kind: OperatorQuery}.toBytes(),
			testOperator{name: "b", kind: OperatorMutation}.toBytes()...,
		),
	)

	injectCodeSurviveTest(query, [][]byte{{'\r'}})
}

func TestParseQueryWithField(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{
			some_field
		}`,
		testOperator{
			fields: []testField{
				{name: "some_field"},
			},
		}.toBytes(),
	)
}

func TestParseQueryWithMultipleFields(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{
			some_field
		}`,
		testOperator{
			fields: []testField{
				{name: "some_field"},
			},
		}.toBytes(),
	)

	testCases := []struct {
		name  string
		query string
	}{
		{"normal human query", `{
			some_field
			other
		}`},
		{"human query with commas in beteween", `{
			some_field,
			other
		}`},
		{"insane people query", `query {
			some_field ,
			other      ,
		}`},
		{"inline test 1", `{some_field other}`},
		{"inline test 2", `{some_field,other}`},
		{"inline test 3", `{some_field,other,}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			newParseQueryAndExpectResult(
				t,
				testCase.query,
				testOperator{
					fields: []testField{
						{name: "some_field"},
						{name: "other"},
					},
				}.toBytes(),
			)
		})
	}
}

func TestParseQueryWithFieldWithSelectionSet(t *testing.T) {
	testCases := []struct {
		name  string
		query string
	}{
		{"human", `{
			some_field {
				foo
				bar
			}
		}`},
		{"human with commas", `{
			some_field {
				foo,
				bar,
			},
		}`},
		{"insane human query", `{
			some_field {
				foo  ,
				bar  ,
			}        ,
		}`},
		{"inline #1", `{some_field{foo bar}}`},
		{"inline #2", `{some_field{foo,bar}}`},
		{"inline #3", `{some_field{foo,bar,},}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			newParseQueryAndExpectResult(
				t,
				testCase.query,
				testOperator{
					fields: []testField{
						{
							name: "some_field",
							fields: []testField{
								{name: "foo"},
								{name: "bar"},
							},
						},
					},
				}.toBytes(),
			)
		})
	}
}

func TestParseQueryWithFieldWithFragmentSpread(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{
			some_field {
				foo
				... baz
				bar
			}
		}`,
		testOperator{
			fields: []testField{
				{name: "some_field", fields: []testField{
					{name: "foo"},
					{name: "baz", isFragment: true},
					{name: "bar"},
				}},
			},
		}.toBytes(),
	)

	// A query that starts with "on" should parse as a fragment pointer
	newParseQueryAndExpectResult(
		t,
		`{
			some_field {
				foo
				... online
				bar
			}
		}`,
		testOperator{
			fields: []testField{
				{name: "some_field", fields: []testField{
					{name: "foo"},
					{name: "online", isFragment: true},
					{name: "bar"},
				}},
			},
		}.toBytes(),
	)
}

func TestParseQueryWithFieldWithInlineFragmentSpread(t *testing.T) {
	testCases := []struct {
		name  string
		query string
	}{
		{"human", `{
			some_field {
				foo
				... on baz {
					bazField
				}
				bar
			}
		}`},
		{"human with commas", `{
			some_field {
				foo,
				... on baz {
					bazField,
				},
				bar,
			},
		}`},
		{"inline #1", `{some_field{foo ...on baz{bazField}bar}}`},
		{"inline #2", `{some_field{foo,...on baz{bazField},bar}}`},
		{"inline #1", `{some_field{foo,...on baz{bazField,},bar,}}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			newParseQueryAndExpectResult(
				t,
				testCase.query,
				testOperator{
					fields: []testField{
						{
							name: "some_field",
							fields: []testField{
								{name: "foo"},
								{
									name:       "baz",
									isFragment: true,
									fields: []testField{
										{name: "bazField"},
									},
								},
								{name: "bar"},
							},
						},
					},
				}.toBytes(),
			)
		})
	}
}

func TestParseAlias(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{
			foo: baz
		}`,
		testOperator{
			fields: []testField{
				{
					name:  "baz",
					alias: "foo",
				},
			},
		}.toBytes(),
	)

	injectCodeSurviveTest(`{foo:baz}`)
}

func TestParseArgumentsWithoutInput(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{
			baz()
		}`,
		testOperator{
			fields: []testField{
				{
					name:      "baz",
					arguments: []typeObjectValue{},
				},
			},
		}.toBytes(),
	)

	injectCodeSurviveTest(`{baz()}`)
}

func TestParseArgumentValueTypes(t *testing.T) {
	options := []struct {
		name   string
		input  string
		output testValue
	}{
		{"bool true", `true`, testValue{kind: ValueBoolean, boolValue: true}},
		{"bool false", `false`, testValue{kind: ValueBoolean, boolValue: false}},
		{"null", `null`, testValue{kind: ValueNull}},
		{"variable", `$banana`, testValue{kind: ValueVariable, variableValue: `banana`}},
		{"enum", `BANANA`, testValue{kind: ValueEnum, enumValue: `BANANA`}},
		{"int", `10`, testValue{kind: ValueInt, intValue: 10}},
		{"int negative", `-20`, testValue{kind: ValueInt, intValue: -20}},
		{"float #1", `10.1`, testValue{kind: ValueFloat, floatValue: `10.1`}},
		{"float #2", `-20.1`, testValue{kind: ValueFloat, floatValue: `-20.1`}},
		{"float #3", `10.1E3`, testValue{kind: ValueFloat, floatValue: `10.1E3`}},
		{"float #4", `-20.1e-3`, testValue{kind: ValueFloat, floatValue: `-20.1E-3`}},
		{"string", `"abc"`, testValue{kind: ValueString, stringValue: `abc`}},
		{"block string", `"""abc"""`, testValue{kind: ValueString, stringValue: `abc`}},
		{"block string with new lines", `"""abc` + "\n" + `abc` + "\r\n" + `abc"""`, testValue{kind: ValueString, stringValue: "abc\nabc\r\nabc"}},
		{"empty string", `""`, testValue{kind: ValueString}},
		{"empty block string", `""""""`, testValue{kind: ValueString}},
		{"string with special", `"\b"`, testValue{kind: ValueString, stringValue: "\b"}},
		{"string with ascii encoded char", `"a\u0021b"`, testValue{kind: ValueString, stringValue: "a!b"}},
		{"string with utf8 encoded char", `"a\u03A3b"`, testValue{kind: ValueString, stringValue: "aΣb"}},
		{"empty object", `{}`, testValue{kind: ValueObject, objectValue: []typeObjectValue{}}},
		{"empty list", `[]`, testValue{kind: ValueList, list: []testValue{}}},
		{
			"object with kv",
			`{a: true}`,
			testValue{
				kind: ValueObject,
				objectValue: []typeObjectValue{
					{
						name:  "a",
						value: testValue{kind: ValueBoolean, boolValue: true},
					},
				},
			},
		},
		{
			"",
			`[true, false]`,
			testValue{
				kind: ValueList,
				list: []testValue{
					{kind: ValueBoolean, boolValue: true},
					{kind: ValueBoolean, boolValue: false},
				},
			},
		},
		{
			"",
			`[true false]`,
			testValue{
				kind: ValueList,
				list: []testValue{
					{kind: ValueBoolean, boolValue: true},
					{kind: ValueBoolean, boolValue: false},
				},
			},
		},
	}

	for _, option := range options {
		t.Run(option.name, func(t *testing.T) {
			query := `{baz(foo: ` + option.input + `)}`
			newParseQueryAndExpectResult(
				t,
				query,
				testOperator{
					fields: []testField{
						{
							name: "baz",
							arguments: []typeObjectValue{
								{
									name:  "foo",
									value: option.output,
								},
							},
						},
					},
				}.toBytes(),
			)

			injectCodeSurviveTest(query, [][]byte{{'+'}, {'.'}, {'e'}, {'"'}, {0}, {'\n'}, {'\r'}})
		})
	}

	// To improve code cov
	injectCodeSurviveTest(`query {baz(foo: "\a\b\c")}`, [][]byte{{'b'}, {'f'}, {'n'}, {'r'}, {'t'}, {'u'}, {'\b'}, {'\f'}, {'\n'}, {'\r'}, {'\t'}})
}

func TestParseMultipleArguments(t *testing.T) {
	testCases := []struct {
		name  string
		query string
	}{
		{"human", `
			{
				baz(
					foo: true
					bar: false
				)
			}
		`},
		{"sane inline", `{baz(foo: true, bar: false)}`},
		{"inline without commas", `{baz(foo: true bar: false)}`},
		{"inline with many commas", `{baz(foo:true,bar:false,)}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			newParseQueryAndExpectResult(
				t,
				testCase.query,
				testOperator{
					fields: []testField{
						{
							name: "baz",
							arguments: []typeObjectValue{
								{
									name:  "foo",
									value: testValue{kind: ValueBoolean, boolValue: true},
								},
								{
									name:  "bar",
									value: testValue{kind: ValueBoolean, boolValue: false},
								},
							},
						},
					},
				}.toBytes(),
			)
		})
	}
}

func TestParseFragment(t *testing.T) {
	query := `fragment Foo on Bar {}`

	newParseQueryAndExpectResult(
		t,
		query,
		testFragment{
			name:   "Foo",
			on:     "Bar",
			fields: []testField{},
		}.toBytes(),
	)

	injectCodeSurviveTest(query)
}

func TestParseFragmentWithFields(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`fragment Foo on Bar {
			fieldA
			bField
		}`,
		testFragment{
			name: "Foo",
			on:   "Bar",
			fields: []testField{
				{name: "fieldA"},
				{name: "bField"},
			},
		}.toBytes(),
	)
}

func TestParseOperatonDirective(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`query a @banana @peer {}`,
		testOperator{
			name: "a",
			directives: []testDirective{
				{name: "banana"},
				{name: "peer"},
			},
		}.toBytes(),
	)
}

func TestParseOperatonDirectiveWithArgs(t *testing.T) {
	query := `query a @banana(a: 1, b: 2) {}`

	newParseQueryAndExpectResult(
		t,
		query,
		testOperator{
			name: "a",
			directives: []testDirective{
				{
					name: "banana",
					arguments: []typeObjectValue{
						{
							name:  "a",
							value: testValue{kind: ValueInt, intValue: 1},
						},
						{
							name:  "b",
							value: testValue{kind: ValueInt, intValue: 2},
						},
					},
				},
			},
		}.toBytes(),
	)

	injectCodeSurviveTest(query)
}

func TestParseQueryWithFieldDirective(t *testing.T) {
	query := `{
		some_field @banana
	}`

	newParseQueryAndExpectResult(
		t,
		query,
		testOperator{fields: []testField{{
			name:       "some_field",
			directives: []testDirective{{name: "banana"}},
		}}}.toBytes(),
	)

	injectCodeSurviveTest(query)
}

func TestParseQueryWithFragmentDirective(t *testing.T) {
	// Inline fragment
	query := `{... on baz @foo {}}`
	newParseQueryAndExpectResult(
		t,
		query,
		testOperator{fields: []testField{{
			name:       "baz",
			isFragment: true,
			fields:     []testField{},
			directives: []testDirective{{name: "foo"}},
		}}}.toBytes(),
	)
	injectCodeSurviveTest(query)

	// Pointer to fragment
	query = `{...baz@foo}`
	newParseQueryAndExpectResult(
		t,
		query,
		testOperator{fields: []testField{{
			name:       "baz",
			isFragment: true,
			directives: []testDirective{{name: "foo"}},
		}}}.toBytes(),
	)
	injectCodeSurviveTest(query)
}

func TestParseLotsOfFieldArguments(t *testing.T) {
	newParseQueryAndExpectResult(
		t,
		`{
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
			)
		}`,
		testOperator{
			fields: []testField{{
				name: "foo",
				arguments: []typeObjectValue{
					{name: "string", value: testValue{kind: ValueString, stringValue: "abc"}},
					{name: "int", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "int8", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "int16", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "int32", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "int64", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "uint", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "uint8", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "uint16", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "uint32", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "uint64", value: testValue{kind: ValueInt, intValue: 123}},
					{name: "bool", value: testValue{kind: ValueBoolean, boolValue: true}},
				},
			}},
		}.toBytes(),
	)
}

func TestMoreThan255Directives(t *testing.T) {
	parseQueryAndExpectErr(t, `{bar`+strings.Repeat(" @foo", 256)+`}`, "cannot have more than 255 directives")
}

// tests if parser doesn't panic nor hangs on wired inputs
func injectCodeSurviveTest(baseQuery string, extraChars ...[][]byte) {
	toTest := [][][]byte{
		{{}, {'_'}, {'-'}, {'0'}},
		{{';'}, {' '}, {'#'}, []byte(" - ")},
		{{'['}, {']'}, {'{'}, {'}'}},
		{{'('}, {'}'}, {'a'}, {','}},
	}

	if len(extraChars) > 0 {
		addToIdx := 0
		for _, set := range extraChars {
			for _, extra := range set {
				toTest[addToIdx] = append(toTest[addToIdx], extra)
				addToIdx++
				if addToIdx == len(toTest) {
					addToIdx = 0
				}
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(toTest))

	for _, charsToInject := range toTest {
		go func(baseQuery []byte, charsToInject [][]byte) {
			parser := ParserCtx{
				Res:               []byte{},
				FragmentLocations: []int{},
				Query:             []byte{},
				Errors:            []error{},
				Hasher:            fnv.New32(),
			}

			for i := range baseQuery {
				tilIndex := baseQuery[:i]
				formIndex := baseQuery[i:]

				parser.Query = append(parser.Query[:0], formIndex...)
				parser.ParseQueryToBytecode(nil)

				for _, toInject := range charsToInject {
					parser.Query = append(parser.Query[:0], tilIndex...)
					parser.Query = append(parser.Query, toInject...)
					parser.ParseQueryToBytecode(nil)
				}

				for _, toInject := range charsToInject {
					parser.Query = append(parser.Query[:0], tilIndex...)
					parser.Query = append(parser.Query, toInject...)
					l := len(parser.Query)

					// Inject extra char(s)
					parser.Query = append(parser.Query, formIndex...)
					parser.ParseQueryToBytecode(nil)

					// Replace char(s)
					parser.Query = append(parser.Query[:l], baseQuery[i+1:]...)
					parser.ParseQueryToBytecode(nil)
				}
			}
			wg.Done()
		}([]byte(baseQuery), charsToInject)
	}

	wg.Wait()
}

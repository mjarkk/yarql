package graphql

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func parseQuery(query string) ([]byte, []error) {
	i := parserCtx{
		res:    []byte{},
		query:  []byte(query),
		errors: []error{},
	}
	i.parseQueryToBytecode()
	return i.res, i.errors
}

func formatHumanReadableQuery(result string) string {
	result = strings.TrimSpace(result)
	result = strings.ReplaceAll(result, " ", "")
	result = strings.ReplaceAll(result, "\t", "")
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = strings.Split(line, "//")[0]
	}
	return strings.Join(lines, "\n")
}

func formatResToHumandReadable(result []byte) string {
	result = bytes.Join(bytes.Split(result, []byte{0}), []byte{'\n'})
	return strings.TrimSpace(string(result))
}

// parseQueryAndExpectResult is a readable tester for the bytecode
// The `expectedResult` is formatted like so:
// - Enter == null byte
// - Spaces characters are removed ('\t', ' ')
// - Comments can be made using // and will be trimmed away in the output
func parseQueryAndExpectResult(t *testing.T, query, expectedResult string) {
	res, errs := parseQuery(query)
	for _, err := range errs {
		panic(err.Error())
	}
	Equal(t, formatHumanReadableQuery(expectedResult), formatResToHumandReadable(res), query)
}

func parseQueryAndExpectErr(t *testing.T, query, expectedErr string) {
	_, errs := parseQuery(query)
	if len(errs) == 0 {
		Fail(t, "exected query to fail with error: "+expectedErr, query)
	}
	Equal(t, errs[0].Error(), expectedErr)
}

func TestParseSimpleQuery(t *testing.T) {
	parseQueryAndExpectResult(t, `{}`, `
		oqf // [operator] [query]
		// No directives
		e  // [end of operator]
	`)
}

func TestParseSimpleQueryWrittenOut(t *testing.T) {
	parseQueryAndExpectResult(t, `query {}`, `
		oqf // operator of type query
		// No directives
		e  // end of operator
	`)
}

func TestParseSimpleMutation(t *testing.T) {
	parseQueryAndExpectResult(t, `mutation {}`, `
		omf // operator of type mutation
		// No directives
		e  // end of operator
	`)
}

func TestParseSimpleSubscription(t *testing.T) {
	parseQueryAndExpectResult(t, `subscription {}`, `
		osf // operator of type subscription
		// No directives
		e  // end of operator
	`)
}

func TestParseQueryWithName(t *testing.T) {
	parseQueryAndExpectResult(t, `query banana {}`, `
		oqf      // operator of type query
		banana   // operator name
		e        // end of operator
	`)
}

func TestParseQuerywithArgs(t *testing.T) {
	parseQueryAndExpectResult(t, `query banana(quality: [Int]) {}`, `
		oqt      // operator of type query
		banana   // operator name
		A        // operator args
        aquality // argument with name banana
        lnInt    // argument of type list with an inner type Int
		f        // this argument has default values
		e        // end of operator arguments
		e        // end of operator
	`)

	query := `query banana(quality: [Int!]! = [10]) {}`

	parseQueryAndExpectResult(t, query, `
		oqt      // operator of type query
		banana   // operator name
		A        // operator args
        aquality // argument with name banana
        LNInt    // argument of type required list with an inner type Int also required
		t        // this argument has default values
		vl       // list value
		vi10     // value of type int with value 10
		e        // end of list value
		e        // end of operator arguments
		e        // end of operator
	`)

	injectCodeSurviveTest(query)
}

func TestParseMultipleSimpleQueries(t *testing.T) {
	parseQueryAndExpectResult(t, `{}{}`, `
		oqf // operator 1
		// No directives
		e  // end of operator 1
		oqf // operator 2
		// No directives
		e  // end of operator 2
	`)
}

func TestParseMultipleQueries(t *testing.T) {
	query := `
		query a {}
		mutation b {}
	`

	parseQueryAndExpectResult(t, query, `
		oqf // query operator 1
		a   // operator 1 name
		e   // end of operator 1
		omf // mutation operator 2
		b   // operator 2 name
		e   // end of operator 2
	`)

	injectCodeSurviveTest(query, [][]byte{{'\r'}})
}

func TestParseQueryWithField(t *testing.T) {
	parseQueryAndExpectResult(t, `query {
		some_field
	}`, `
		oqf          // query operator
		// No directives
		f           // start new field
		some_field  // field name
		// no field alias
		e           // end field
		e           // end operator
	`)
}

func TestParseQueryWithMultipleFields(t *testing.T) {
	expectedOutput := `
		oqf          // query operator
		// No directives
		f           // start new field
		some_field  // field name
		// no field alias
		e           // end field with name some_field
		f           // start new field
		other       // field name
		// no field alias
		e           // end field with name other
		e           // end operator
	`

	parseQueryAndExpectResult(t, `query {
		some_field
		other
	}`, expectedOutput)

	parseQueryAndExpectResult(t, `query {
		some_field,
		other
	}`, expectedOutput)

	parseQueryAndExpectResult(t, `query {
		some_field ,
		other      ,
	}`, expectedOutput)
}

func TestParseQueryWithFieldWithSelectionSet(t *testing.T) {
	parseQueryAndExpectResult(t, `query {
		some_field {
			foo
			bar
		}
	}`, `
		oqf          // query operator
		// No directives
		f           // new field
		some_field  // field name
		// no field alias
		f           // new field
		foo         // field name
		// no field alias
		e           // end of foo
		f           // new field
		bar         // field name
		// no field alias
		e           // end of bar
		e           // end of some_field
		e           // end operator
	`)
}

func TestParseQueryWithFieldWithFragmentSpread(t *testing.T) {
	parseQueryAndExpectResult(t, `query {
		some_field {
			foo
			... baz
			bar
		}
	}`, `
		oqf
		// No directives
		f
		some_field
		// no field alias
		f
		foo
		// no field alias
		e
		sf  // fragment spread pointing
		baz // fragment name
		f
		bar
		// no field alias
		e
		e
		e
	`)

	// A query that starts with "on" should parse as a fragment pointer
	parseQueryAndExpectResult(t, `query {
		some_field {
			foo
			... online
			bar
		}
	}`, `
		oqf
		// No directives
		f
		some_field
		// no field alias
		f
		foo
		// no field alias
		e
		sf     // fragment spread pointing
		online // fragment name
		f
		bar
		// no field alias
		e
		e
		e
	`)

}

func TestParseQueryWithFieldWithInlineFragmentSpread(t *testing.T) {
	expectedOutput := `
		oqf
		// No directives
		f
		some_field
		// no field alias
		f
		foo
		// no field alias
		e
		st       // inline fragment spread
		baz      // fragment name
		f
		bazField // fragment field
		// no field alias
		e         // end of fragment field
		e         // end of inline fragment
		f
		bar
		// no field alias
		e
		e
		e
	`

	parseQueryAndExpectResult(t, `query {
		some_field {
			foo
			... on baz {
				bazField
			}
			bar
		}
	}`, expectedOutput)

	parseQueryAndExpectResult(t, `query {
		some_field {
			foo,
			... on baz {
				bazField
			},
			bar,
		}
	}`, expectedOutput)
}

func TestParseAlias(t *testing.T) {
	query := `query {
		foo: baz
	}`

	parseQueryAndExpectResult(t, query, `
		oqf
		// No directives
		f
		foo // field with alias foo
		baz  // field name
		e    // end of field
		e
	`)

	injectCodeSurviveTest(query)
}

func TestParseArgumentsWithoutInput(t *testing.T) {
	query := `query {
		baz()
	}`

	parseQueryAndExpectResult(t, query, `
		oqf
		// No directives
		f
		baz // field with alias foo
		// no alias
		vo   // value of kind object (these are the arguments)
		e    // end of value object / arguments
		e    // end of field
		e
	`)

	injectCodeSurviveTest(query)
}

func TestParseArgumentValueTypes(t *testing.T) {
	options := []struct {
		input  string
		output string
	}{
		{`true`, `vb1`},            // boolean
		{`false`, `vb0`},           // boolean
		{`null`, `vn`},             // null
		{`$banana`, `v$banana`},    // variable reference
		{`BANANA`, `veBANANA`},     // Enum
		{`10`, `vi10`},             // Int
		{`-20`, `vi-20`},           // Int
		{`10.1`, `vf10.1`},         // Float
		{`-20.1`, `vf-20.1`},       // Float
		{`10.1E3`, `vf10.1E3`},     // Float
		{`-20.1e-3`, `vf-20.1E-3`}, // Float
		{`"abc"`, `vsabc`},         // String
		{`"""abc"""`, `vsabc`},     // String (block string)
		{`"""abc` + "\n" + `abc` + "\r\n" + `abc"""`, "vsabc\nabc\r\nabc"}, // String (block string)
		{`""`, `vs`},                         // String
		{`""""""`, `vs`},                     // String (block string)
		{`"\b"`, "vs\b"},                     // String
		{`"a\u0021b"`, "vsa!b"},              // String
		{`"a\u03A3b"`, "vsaΣb"},              // String
		{`{}`, "vo\ne"},                      // Object
		{`[]`, "vl\ne"},                      // List
		{`{a: true}`, "vo\nua\nvb1\ne"},      // Object
		{`[true, false]`, "vl\nvb1\nvb0\ne"}, // List
		{`[true false]`, "vl\nvb1\nvb0\ne"},  // List
	}

	for _, option := range options {
		query := `query {baz(foo: ` + option.input + `)}`
		parseQueryAndExpectResult(t, query, `
			oqf
			// No directives
			f
			baz  // field with alias foo
			// no alias
			vo   // value of kind object (these are the arguments)
			ufoo // key foo
			`+option.output+`
			e    // end of value object / arguments
			e    // end of field
			e
		`)

		injectCodeSurviveTest(query, [][]byte{{'+'}, {'.'}, {'e'}, {'"'}, {0}, {'\n'}, {'\r'}})
	}

	// To improve code cov
	injectCodeSurviveTest(`query {baz(foo: "\a\b\c")}`, [][]byte{{'b'}, {'f'}, {'n'}, {'r'}, {'t'}, {'u'}, {'\b'}, {'\f'}, {'\n'}, {'\r'}, {'\t'}})
}

func TestParseMultipleArguments(t *testing.T) {
	expect := `
		oqf
		// No directives
		f
		baz // field with alias foo
		// no alias
		vo   // value of kind object (these are the arguments)
		ufoo // key foo
		vb1  // boolean value with data true
		ubar // key bar
		vb0  // boolean value with data false
		e    // end of value object / arguments
		e    // end of field
		e
	`

	parseQueryAndExpectResult(t, `query {baz(foo: true, bar: false)}`, expect)
	parseQueryAndExpectResult(t, `query {baz(foo: true bar: false)}`, expect)
	parseQueryAndExpectResult(t, `query {baz(foo: true, bar: false,)}`, expect)
}

func TestParseFragment(t *testing.T) {
	query := `fragment Foo on Bar {}`

	parseQueryAndExpectResult(t, query, `
		FFoo // fragment with name Foo
		Bar  // fragment type name
		e    // end of fragment
	`)

	injectCodeSurviveTest(query)
}

func TestParseFragmentWithFields(t *testing.T) {
	parseQueryAndExpectResult(t, `fragment Foo on Bar {
		fieldA
		bField
	}`, `
		FFoo    // fragment with name Foo
		Bar     // fragment type name
		f
		fieldA
		// no field alias
		e
		f
		bField
		// no field alias
		e
		e       // end of fragment
	`)
}

func TestParseOperatonDirective(t *testing.T) {
	parseQueryAndExpectResult(t, `query a @banana @peer {}`, `
		oqf`+"\x02"+`a // new operator with name a and 2 directives
		dfbanana       // directive with no arguments and name banana
		dfpeer         // directive with no arguments and name peer
		e              // end of operator
	`)
}

func TestParseOperatonDirectiveWithArgs(t *testing.T) {
	query := `query a @banana(a: 1, b: 2) {}`
	parseQueryAndExpectResult(t, query, `
		oqf`+"\x01"+`a // new operator with name a and 2 directives
		dtbanana       // directive with arguments and name banana
		vo             // value with type object (start directive arguments values)
        ua             // object field a
        vi1            // value type int with value 1
        ub             // object field b
        vi2            // value type int with value 2
		e              // end object (end directive arguments)
		e              // end of operator
	`)

	injectCodeSurviveTest(query)
}

func TestParseQueryWithFieldDirective(t *testing.T) {
	query := `query {
		some_field @banana
	}`

	parseQueryAndExpectResult(t, query, `
		oqf
		// No directives
		f`+"\x01"+`some_field // field with 1 arguments
		// no field alias
		dfbanana              // directive banana with no arguments
		e
		e
	`)

	injectCodeSurviveTest(query)
}

func TestParseQueryWithFragmentDirective(t *testing.T) {
	// Inline fragment
	query := `{... on baz @foo {}}`
	parseQueryAndExpectResult(t, query, `
		oqf

		st`+"\x01"+`baz // fragment with 1 directive
		dffoo           // directive with name foo and no arguments
		e
		e
	`)
	injectCodeSurviveTest(query)

	// Pointer to fragment
	query = `{...baz@foo}`
	parseQueryAndExpectResult(t, query, `
		oqf

		sf`+"\x01"+`baz // fragment with 1 directive
		dffoo           // directive with name foo and no arguments
		e
	`)
	injectCodeSurviveTest(query)
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
			parser := parserCtx{
				res:    []byte{},
				query:  []byte{},
				errors: []error{},
			}

			for i := range baseQuery {
				tilIndex := baseQuery[:i]
				formIndex := baseQuery[i:]

				parser.query = append(parser.query[:0], formIndex...)
				parser.parseQueryToBytecode()

				for _, toInject := range charsToInject {
					parser.query = append(parser.query[:0], tilIndex...)
					parser.query = append(parser.query, toInject...)
					parser.parseQueryToBytecode()
				}

				for _, toInject := range charsToInject {
					parser.query = append(parser.query[:0], tilIndex...)
					parser.query = append(parser.query, toInject...)
					l := len(parser.query)

					// Inject extra char(s)
					parser.query = append(parser.query, formIndex...)
					parser.parseQueryToBytecode()

					// Replace char(s)
					parser.query = append(parser.query[:l], baseQuery[i+1:]...)
					parser.parseQueryToBytecode()
				}
			}
			wg.Done()
		}([]byte(baseQuery), charsToInject)
	}

	wg.Wait()
}

package graphql

import (
	"bytes"
	"strings"
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
	result = strings.ReplaceAll(result, "\r", "")
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
// - Spaces characters are removed ('\r','\t',' ')
// - Comments can be made using // and will be trimmed away in the output
func parseQueryAndExpectResult(t *testing.T, query, expectedResult string) {
	res, errs := parseQuery(query)
	for _, err := range errs {
		panic(err.Error())
	}
	Equal(t, formatHumanReadableQuery(expectedResult), formatResToHumandReadable(res), query)
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

	parseQueryAndExpectResult(t, `query banana(quality: [Int!]! = [10]) {}`, `
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
	parseQueryAndExpectResult(t, `
		query a {}
		mutation b {}
	`, `
		oqf // query operator 1
		a   // operator 1 name
		e   // end of operator 1
		omf // mutation operator 2
		b   // operator 2 name
		e   // end of operator 2
	`)
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
		sfbaz // fragment spread pointing to fragment with name baz
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
		sfonline // fragment spread pointing to fragment with name online
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
		stbaz     // fragment spread with typename baz
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
	parseQueryAndExpectResult(t, `query {
		foo: baz
	}`, `
		oqf
		// No directives
		f
		foo // field with alias foo
		baz  // field name
		e    // end of field
		e
	`)
}

func TestParseArgumentsWithoutInput(t *testing.T) {
	parseQueryAndExpectResult(t, `query {
		baz()
	}`, `
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
}

func TestParseArgumentValueTypes(t *testing.T) {
	options := []struct {
		input  string
		output string
	}{
		{`true`, `vb1`},                      // boolean
		{`false`, `vb0`},                     // boolean
		{`null`, `vn`},                       // null
		{`$banana`, `v$banana`},              // variable reference
		{`BANANA`, `veBANANA`},               // Enum
		{`10`, `vi10`},                       // Int
		{`-20`, `vi-20`},                     // Int
		{`10.1`, `vf10.1`},                   // Float
		{`-20.1`, `vf-20.1`},                 // Float
		{`10.1E3`, `vf10.1E3`},               // Float
		{`-20.1e-3`, `vf-20.1E-3`},           // Float
		{`{}`, "vo\ne"},                      // Object
		{`[]`, "vl\ne"},                      // List
		{`{a: true}`, "vo\nua\nvb1\ne"},      // Object
		{`[true, false]`, "vl\nvb1\nvb0\ne"}, // List
		{`[true false]`, "vl\nvb1\nvb0\ne"},  // List
	}

	for _, option := range options {
		parseQueryAndExpectResult(t, `query {baz(foo: `+option.input+`)}`, `
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
	}
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
	parseQueryAndExpectResult(t, `fragment Foo on Bar {}`, `
		FFoo // fragment with name Foo
		Bar  // fragment type name
		e    // end of fragment
	`)
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

package graphql

import (
	"reflect"
	"strconv"
	"sync"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func checkErrorHaveLocation(err *ErrorWLocation) {
	if err.column == 0 {
		panic("error columns should not be 0")
	}
	if err.err == nil {
		panic("error should have an error within")
	}
}

func TestQueryParserEmptyQuery(t *testing.T) {
	res, err := parseQuery(``)
	Equal(t, 0, len(res))
	Nil(t, err)

	res, err = parseQuery(`  `)
	Equal(t, 0, len(res))
	Nil(t, err)
}

func TestQueryParserEmptyBracesQuery(t *testing.T) {
	options := []struct {
		query                 string
		expectedOperationType string
		shouldFail            bool
	}{
		{query: "{}", expectedOperationType: "query"},
		{query: "query {}", expectedOperationType: "query"},
		{query: "mutation {}", expectedOperationType: "mutation"},
		{query: "subscription {}", expectedOperationType: "subscription"},
		{query: "query{}", expectedOperationType: "query"},
		{query: "query\n{}", expectedOperationType: "query"},
		{query: "query\r\n{}", expectedOperationType: "query"},
		{query: "query\t{}", expectedOperationType: "query"},
		{query: "query\t \n\r\n{}", expectedOperationType: "query"},
		{query: "     {    }    ", expectedOperationType: "query"},
		{query: "}", shouldFail: true},
		{query: "{", shouldFail: true},
		{query: "invalidValue {}", shouldFail: true},
		{query: "invalidValue{}", shouldFail: true},
		{query: "i{}", shouldFail: true},
	}

	for _, option := range options {
		res, err := parseQuery(option.query)
		if option.shouldFail {
			NotNil(t, err, option.query)
		} else {
			Equal(t, 1, len(res), option.query)
			Nil(t, err, option.query)
			Equal(t, option.expectedOperationType, res[0].operationType, option.query)
			Equal(t, "", res[0].name, option.query)
		}
	}
}

func TestQueryParserEmptyBracesQueryWithName(t *testing.T) {
	options := []struct {
		query                 string
		expectedOperationType string
	}{
		{query: "query name_here {}", expectedOperationType: "query"},
		{query: "mutation name_here {}", expectedOperationType: "mutation"},
		{query: "subscription name_here {}", expectedOperationType: "subscription"},
	}

	for _, option := range options {
		res, err := parseQuery(option.query)
		Equal(t, 1, len(res), option.query)
		Nil(t, err, option.query)
		Equal(t, option.expectedOperationType, res[0].operationType, option.query)
		Equal(t, "name_here", res[0].name, option.query)
	}
}

func TestQueryParsingQueryDirectives(t *testing.T) {
	res, err := parseQuery(`query foo @bar {}`)
	Equal(t, 1, len(res))
	Nil(t, err)

	res, err = parseQuery(`query @bar {}`)
	Equal(t, 1, len(res))
	Nil(t, err)
}

func TestQueryParserMultipleQueries(t *testing.T) {
	res, err := parseQuery(`
		query a {}
		query b {}
		query c {}
		query d {}
	`)
	Equal(t, 4, len(res))
	Nil(t, err)

	for i, expectedName := range []string{"a", "b", "c", "d"} {
		Equal(t, expectedName, res[i].name)
	}
}

func TestQueryParsingTypes(t *testing.T) {
	toTest := []string{
		`query ($a: String) {}`,
		`query ($a: [String]) {}`,
		`query ($a: [String!]) {}`,
		`query ($a: Boolean = true) {}`,
		`query ($a: Null = null) {}`,
	}

	for _, item := range toTest {
		res, err := parseQuery(item)
		Equal(t, 1, len(res), item)
		Nil(t, err, item)
	}

	res, err := parseQuery(`query query_name( $a : String $b:Boolean) {}`)
	Equal(t, 1, len(res))
	Nil(t, err)
	Equal(t, 2, len(res[0].variableDefinitions))
	item1 := res[0].variableDefinitions["a"]
	item2 := res[0].variableDefinitions["b"]
	Equal(t, "String", item1.varType.name)
	Equal(t, "Boolean", item2.varType.name)
	Nil(t, item1.defaultValue)
	Nil(t, item2.defaultValue)

}

func TestQueryParserNumbers(t *testing.T) {
	options := []struct {
		isInt bool
		input string
	}{
		{isInt: true, input: "0"},
		{isInt: true, input: "1"},
		{isInt: true, input: "10"},
		{isInt: true, input: "-10"},
		{input: "1.2"},
		{input: "11.22"},
		{input: "-1.2"},
		{input: "1e3"},
		{input: "11e33"},
		{input: "1E3"},
		{input: "-1e3"},
		{input: "11.2e3"},
		{input: "11.2E3"},
		{input: "-11.2e3"},
		{input: "-11.22e+33"},
	}

	for _, option := range options {
		name := "Float"
		if option.isInt {
			name = "Int"
		}

		query := `query ($b: ` + name + ` = ` + option.input + `) {}`
		res, err := parseQuery(query)
		Equal(t, 1, len(res), option.input)
		Nil(t, err, option.input)

		item := res[0].variableDefinitions["b"]

		if option.isInt {
			Equal(t, reflect.Int, item.defaultValue.valType, option.input)
			n, _ := strconv.Atoi(option.input)
			Equal(t, n, item.defaultValue.intValue, option.input)
		} else {
			Equal(t, reflect.Float64, item.defaultValue.valType, option.input)
			f, _ := strconv.ParseFloat(option.input, 64)
			Equal(t, f, item.defaultValue.floatValue, option.input)
		}

		injectCodeSurviveTest(query, `+`, `.`, `e`)
	}
}

func TestQueryParserStrings(t *testing.T) {
	options := []struct {
		input  string
		output string
	}{
		{input: `""`, output: ""},
		{input: `"abc"`, output: "abc"},
		{input: `"a\nb"`, output: "a\nb"},
		{input: `"a\rb"`, output: "a\rb"},
		{input: `"a\ta"`, output: "a\ta"},
		{input: `"a\bb"`, output: "a\bb"},
		{input: `"a\fa"`, output: "a\fa"},
		{input: `"a \u0021 b"`, output: "a ! b"},
		{input: `"a \u03A3 b"`, output: "a Σ b"},
		{input: `"a \u123 b"`, output: "a u123 b"},
		{input: `"a \u0000 b"`, output: "a  b"},
		{input: `"\""`, output: "\""},
		{input: `""""""`, output: ""},
		{input: `"""abc"""`, output: "abc"},
		{input: `"""a` + "\n" + `b"""`, output: "a\nb"},
		{input: `"""a \""" b"""`, output: "a \"\"\" b"},
	}

	for _, option := range options {
		query := `query ($b: String = ` + option.input + `) {}`
		res, err := parseQuery(query)
		Equal(t, 1, len(res), option.input)
		Nil(t, err, option.input)

		item := res[0].variableDefinitions["b"]

		Equal(t, reflect.String, item.defaultValue.valType, option.input)
		Equal(t, option.output, item.defaultValue.stringValue)

		injectCodeSurviveTest(query, `\u`, `"`, `\`, "\n")
	}
}

func TestQueryParserSimpleInvalid(t *testing.T) {
	_, err := parseQuery(`This should not get parsed`)
	NotNil(t, err)
}

func TestQueryParserSimpleQuery(t *testing.T) {
	options := []string{
		`{
			a
			b
			c: d
			# This is a comment that should not be parsed nor cause an error
		}`,
		`{
			a,
			b,
			c : d,
		}`,
		`{a b c:d}`,
		`{a,b,c:d}`,
	}

	for _, option := range options {
		res, err := parseQuery(option)
		Equal(t, 1, len(res), option)
		Nil(t, err, option)

		Equal(t, 3, len(res[0].selection), "Should have 3 properties")

		selectionMap := map[string]field{}
		for _, item := range res[0].selection {
			Equal(t, "Field", item.selectionType)
			NotNil(t, item.field)
			selectionMap[item.field.name] = *item.field
		}

		Contains(t, selectionMap, "a")
		Contains(t, selectionMap, "b")
		Contains(t, selectionMap, "d")

		injectCodeSurviveTest(option)
	}
}

func TestQueryParserInvalidQuery(t *testing.T) {
	options := []string{
		`{
			a
			\ b
			c
		}`,
		`{a b`,
		`{a-b-c}`,
	}

	for _, option := range options {
		_, err := parseQuery(option)
		NotNil(t, err, option)
		checkErrorHaveLocation(err)
	}
}

func TestQueryParserSelectionInSelection(t *testing.T) {
	res, err := parseQuery(`{
		baz {
			foo
			bar
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	NotEmpty(t, res[0].selection)
	selection := res[0].selection[0]
	field := selection.field

	Equal(t, "Field", selection.selectionType)

	NotNil(t, field)
	Equal(t, "baz", field.name)
	NotNil(t, field.selection)
	Equal(t, 2, len(field.selection))

	selection = field.selection[0]
	NotNil(t, selection.field)
	Equal(t, "foo", selection.field.name)

	selection = field.selection[1]
	Equal(t, "bar", selection.field.name)
}

func TestQueryParserFragmentSpread(t *testing.T) {
	res, err := parseQuery(`{
		baz {
			foo
			...fooBar
			... barFoo
			bar
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	items := res[0].selection[0].field.selection
	Equal(t, 4, len(items))

	Equal(t, "FragmentSpread", items[1].selectionType)
	Equal(t, "FragmentSpread", items[2].selectionType)

	spread1 := items[1].fragmentSpread
	spread2 := items[2].fragmentSpread

	NotNil(t, spread1)
	NotNil(t, spread2)

	Equal(t, "fooBar", spread1.name)
	Equal(t, "barFoo", spread2.name)
}

func TestQueryParserFragmentSpreadDirectives(t *testing.T) {
	res, err := parseQuery(`{
		baz {
			foo
			...fooBar@a@b
			... barFoo @a
			bar
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	items := res[0].selection[0].field.selection

	spread1 := items[1].fragmentSpread
	spread2 := items[2].fragmentSpread

	Equal(t, 2, len(spread1.directives))
	Equal(t, 1, len(spread2.directives))
}

func TestQueryParserInlineFragment(t *testing.T) {
	res, err := parseQuery(`{
		baz {
			foo

			...{

			}

			... on User {
				friends {
					count
				}
			}

			bar
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	items := res[0].selection[0].field.selection
	Equal(t, 4, len(items))

	Equal(t, "InlineFragment", items[1].selectionType)
	Equal(t, "InlineFragment", items[2].selectionType)

	frag1 := items[1].inlineFragment
	frag2 := items[2].inlineFragment

	NotNil(t, frag1)
	NotNil(t, frag2)

	Equal(t, "", frag1.onTypeConditionName)
	Equal(t, "User", frag2.onTypeConditionName)

	NotNil(t, frag2.selection)
	NotEmpty(t, frag2.selection)
}

func TestQueryParserInlineFragmentWithDirectives(t *testing.T) {
	res, err := parseQuery(`{
		baz {
			...@some_directive@a{

			}

			... on User @some_directive {
				friends {
					count
				}
			}
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	items := res[0].selection[0].field.selection

	frag1 := items[0].inlineFragment
	frag2 := items[1].inlineFragment

	Equal(t, 2, len(frag1.directives))
	Equal(t, 1, len(frag2.directives))
}

func TestQueryParserFieldDirective(t *testing.T) {
	res, err := parseQuery(`{
		client {
			foo
			bar @this_is_a_directive
			bas
			baz
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	directives := res[0].selection[0].field.selection[1].field.directives
	NotNil(t, directives)
	Equal(t, 1, len(directives))

	_, ok := directives["this_is_a_directive"]
	True(t, ok)
}

func TestQueryParserFieldInvalidDirective(t *testing.T) {
	_, err := parseQuery(`{
		client {
			foo
			bar @
			bas
			baz
		}
	}`)
	NotNil(t, err)
	checkErrorHaveLocation(err)
}

func TestQueryParserFieldMultipleDirective(t *testing.T) {
	res, err := parseQuery(`{
		client {
			foo
			bar @a @b@c
			bas
			baz
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	directives := res[0].selection[0].field.selection[1].field.directives
	NotNil(t, directives)
	Equal(t, 3, len(directives), "Not all directives")

	expect := []string{"a", "b", "c"}
	for _, item := range expect {
		_, ok := directives[item]
		True(t, ok, "Missing directive: "+item)
	}
}

func TestQueryParserFieldWithArguments(t *testing.T) {
	res, err := parseQuery(`{
		client {
			foo
			bar(a: 1,b:true c : false , d: [1,2 3 , 4,], e: $foo_bar, f: null, g: SomeEnumValue, h: {a: 1, b: true})
			baz()
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	arguments := res[0].selection[0].field.selection[1].field.arguments
	NotNil(t, arguments, "arguments should be defined")

	a, ok := arguments["a"]
	True(t, ok)
	Equal(t, reflect.Int, a.valType)
	Equal(t, 1, a.intValue)

	b, ok := arguments["b"]
	True(t, ok)
	Equal(t, reflect.Bool, b.valType)
	True(t, b.booleanValue)

	c, ok := arguments["c"]
	True(t, ok)
	Equal(t, reflect.Bool, c.valType)
	False(t, c.booleanValue)

	d, ok := arguments["d"]
	True(t, ok)
	Equal(t, reflect.Array, d.valType)
	list := d.listValue
	Equal(t, 4, len(list))

	for i, item := range list {
		Equal(t, reflect.Int, item.valType)
		Equal(t, i+1, item.intValue)
	}
}

func TestQueryParserFieldDirectiveWithArguments(t *testing.T) {
	res, err := parseQuery(`{
		client {
			foo
			bar @a(a: 1,b:true c : false) @b(a: 1,b:true c : false)@c(a: 1,b:true c : false)
			bas
			baz
		}
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))

	directives := res[0].selection[0].field.selection[1].field.directives
	NotNil(t, directives)
	Equal(t, 3, len(directives), "Not all directives")

	expect := []string{"a", "b", "c"}
	for _, item := range expect {
		directive, ok := directives[item]
		True(t, ok, "directive: "+item)
		arguments := directive.arguments

		NotNil(t, arguments, "arguments should be defined")

		a, ok := arguments["a"]
		True(t, ok, "directive: "+item)
		Equal(t, reflect.Int, a.valType, "directive: "+item)
		Equal(t, 1, a.intValue, "directive: "+item)

		b, ok := arguments["b"]
		True(t, ok, "directive: "+item)
		Equal(t, reflect.Bool, b.valType, "directive: "+item)
		True(t, b.booleanValue, "directive: "+item)

		c, ok := arguments["c"]
		True(t, ok, "directive: "+item)
		Equal(t, reflect.Bool, c.valType, "directive: "+item)
		False(t, c.booleanValue, "directive: "+item)
	}
}

// test if:
// - parser doesn't panic on wired inputs
// - parser doesn't hang on certain inputs
func injectCodeSurviveTest(baseQuery string, extraChars ...string) {
	toTest := [][]string{
		{"", "_", "-", "0"},
		{";", " ", "#", " - "},
		{"[", "]", "{", "}"},
		append([]string{"(", ")"}, extraChars...),
	}

	var wg sync.WaitGroup
	wg.Add(len(toTest))

	for _, charsToInject := range toTest {
		go func(baseQuery string, charsToInject []string) {
			for i := range baseQuery {
				tilIndex := baseQuery[:i]
				formIndex := baseQuery[i:]

				parseQuery(formIndex)

				for _, toInject := range charsToInject {
					// Inject extra text
					parseQuery(tilIndex + toInject + formIndex)

					// Replace char
					parseQuery(tilIndex + toInject + baseQuery[i+1:])
				}

				for _, toInject := range charsToInject {
					parseQuery(tilIndex + toInject)
				}
			}
			wg.Done()
		}(baseQuery, charsToInject)
	}

	wg.Wait()
}

func TestQueryParserCodeInjection(t *testing.T) {

	// This function takes quite long
	// possible speed increase could be had by replaceing all /(\s|\t|\n){2,}+/g with " "

	baseQuery := `query client($foo_bar: [Int!]! = 3) @directive_name(a: {a: 1, b: true}) {
		foo
		bar @a @b@c(a: 1,b:true d: [1,2 3 , 4, -11.22e+33], e: $foo_bar, f: null, g: SomeEnumValue, h: {a: 1, b: true c: "", d: """"""}) {
			bar_foo
		}
		...f @a@b
		... on User {
			friends {
				count
			}
		}
		bas
	}`

	injectCodeSurviveTest(baseQuery)
}

func TestQueryParserFragment(t *testing.T) {
	res, err := parseQuery(`fragment a on User {}`)
	Nil(t, err)
	Equal(t, 1, len(res))
	NotNil(t, res[0].fragment)
	Equal(t, "a", res[0].name)
	Equal(t, "User", res[0].fragment.onTypeConditionName)

	res, err = parseQuery(`fragment a on User {
		a
		b
	}`)
	Nil(t, err)
	Equal(t, 1, len(res))
}

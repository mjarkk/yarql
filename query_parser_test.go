package graphql

import (
	"reflect"
	"strconv"
	"strings"
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

func parseQuery(query string) (i iterT, fragments, operators map[string]operator, err *ErrorWLocation) {
	i = newIter(true)

	i.parseQuery(query)
	if len(i.resErrors) > 0 {
		err = &i.resErrors[0]
	}

	fragments = map[string]operator{}
	for _, fragment := range i.fragments {
		fragments[fragment.name] = fragment
	}

	operators = map[string]operator{}
	for _, operator := range i.operators {
		operators[operator.name] = operator
	}

	return i, fragments, operators, err
}

func TestQueryParserEmptyQuery(t *testing.T) {
	_, fragments, operators, err := parseQuery(``)
	Equal(t, 0, len(fragments))
	Equal(t, 0, len(operators))
	Nil(t, err)

	_, fragments, operators, err = parseQuery(`  `)
	Equal(t, 0, len(fragments))
	Equal(t, 0, len(operators))
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
		_, fragments, operators, err := parseQuery(option.query)
		Equal(t, 0, len(fragments))
		if option.shouldFail {
			NotNil(t, err, option.query)
		} else {
			Equal(t, 1, len(operators), option.query)
			Nil(t, err, option.query)
			for _, operator := range operators {
				Equal(t, option.expectedOperationType, operator.operationType, option.query)
				switch option.expectedOperationType {
				case "query":
					True(t, strings.Contains(operator.name, "unknown_query_"), option.query)
				case "mutation":
					True(t, strings.Contains(operator.name, "unknown_mutation_"), option.query)
				case "subscription":
					True(t, strings.Contains(operator.name, "unknown_subscription_"), option.query)
				}
			}
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
		_, fragments, operators, err := parseQuery(option.query)
		Equal(t, 0, len(fragments), option.query)
		Equal(t, 1, len(operators), option.query)
		Nil(t, err, option.query)
		operator, ok := operators["name_here"]
		True(t, ok)
		Equal(t, option.expectedOperationType, operator.operationType, option.query)
	}
}

func TestQueryParsingQueryDirectives(t *testing.T) {
	_, fragments, operators, err := parseQuery(`query foo @bar {}`)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))
	Nil(t, err)

	_, fragments, operators, err = parseQuery(`query @bar {}`)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))
	Nil(t, err)
}

func TestQueryParserMultipleQueries(t *testing.T) {
	_, fragments, operators, err := parseQuery(`
		query a {}
		query b {}
		query c {}
		query d {}
	`)
	Equal(t, 0, len(fragments))
	Equal(t, 4, len(operators))
	Nil(t, err)

	for _, expectedName := range []string{"a", "b", "c", "d"} {
		_, ok := operators[expectedName]
		True(t, ok)
	}
}

func TestQueryParsingTypes(t *testing.T) {
	toTest := []struct {
		query       string
		expectedLen int
	}{
		{`query ($a: String) {}`, 1},
		{`query ($a: [String]) {}`, 1},
		{`query ($a: [String!]) {}`, 1},
		{`query ($a: Boolean = true) {}`, 1},
		{`query ($a: Null = null) {}`, 1},
		{`query ($a: String $b: Int) {}`, 2},
		{`query ($a: String, $b: Int) {}`, 2},
	}

	for _, item := range toTest {
		_, fragments, operators, err := parseQuery(item.query)
		Equal(t, 0, len(fragments), item.query)
		Equal(t, 1, len(operators), item.query)
		for _, operator := range operators {
			Equal(t, item.expectedLen, len(operator.variableDefinitions), item.query)
		}
		Nil(t, err, item.query)
	}

	_, fragments, operators, err := parseQuery(`query query_name( $a : String $b:Boolean) {}`)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))
	Nil(t, err)
	for _, operator := range operators {
		Equal(t, 2, len(operator.variableDefinitions))
		item1 := operator.variableDefinitions["a"]
		item2 := operator.variableDefinitions["b"]
		Equal(t, "String", item1.varType.name)
		Equal(t, "Boolean", item2.varType.name)
		Nil(t, item1.defaultValue)
		Nil(t, item2.defaultValue)
	}
}

func TestQueryParsingInvalidVariableDefinition(t *testing.T) {
	_, _, _, err := parseQuery(`query query_name($a: String = $b) {}`)
	Error(t, err)
	Equal(t, "variables not allowed within this context", err.Error())

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
		_, fragments, operators, err := parseQuery(query)
		Equal(t, 0, len(fragments), option.input)
		Equal(t, 1, len(operators), option.input)
		Nil(t, err, option.input)

		for _, operator := range operators {
			item := operator.variableDefinitions["b"]

			if option.isInt {
				Equal(t, reflect.Int, item.defaultValue.valType, option.input)
				n, _ := strconv.Atoi(option.input)
				Equal(t, n, item.defaultValue.intValue, option.input)
			} else {
				Equal(t, reflect.Float64, item.defaultValue.valType, option.input)
				f, _ := strconv.ParseFloat(option.input, 64)
				Equal(t, f, item.defaultValue.floatValue, option.input)
			}
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
		_, fragments, operators, err := parseQuery(query)
		Equal(t, 0, len(fragments))
		Equal(t, 1, len(operators), option.input)
		Nil(t, err, option.input)

		for _, operator := range operators {
			item := operator.variableDefinitions["b"]

			Equal(t, reflect.String, item.defaultValue.valType, option.input)
			Equal(t, option.output, item.defaultValue.stringValue)
		}

		injectCodeSurviveTest(query, `\u`, `"`, `\`, "\n")
	}
}

func TestQueryParserSimpleInvalid(t *testing.T) {
	_, _, _, err := parseQuery(`This should not get parsed`)
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
		i, fragments, operators, err := parseQuery(option)
		Equal(t, 0, len(fragments), option)
		Equal(t, 1, len(operators), option)
		Nil(t, err, option)

		for _, operator := range operators {
			Equal(t, 3, len(i.selections[operator.selectionIdx]), "Should have 3 properties")

			selectionMap := map[string]field{}
			for _, item := range i.selections[operator.selectionIdx] {
				Equal(t, "Field", item.selectionType)
				selectionMap[item.field.name] = item.field
			}

			Contains(t, selectionMap, "a")
			Contains(t, selectionMap, "b")
			Contains(t, selectionMap, "d")
		}

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
		_, _, _, err := parseQuery(option)
		NotNil(t, err, option)
		checkErrorHaveLocation(err)
	}
}

func TestQueryParserSelectionInSelection(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
		baz {
			foo
			bar
		}
	}`)
	Nil(t, err)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		selection := i.selections[operator.selectionIdx][0]
		field := selection.field

		Equal(t, "Field", selection.selectionType)

		NotNil(t, field)
		Equal(t, "baz", field.name)
		Equal(t, 2, len(i.selections[field.selectionIdx]))

		set := i.selections[field.selectionIdx]
		selection = set[0]
		NotNil(t, selection.field)
		Equal(t, "foo", selection.field.name)

		selection = set[1]
		Equal(t, "bar", selection.field.name)
	}
}

func TestQueryParserFragmentSpread(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
		baz {
			foo
			...fooBar
			... barFoo
			bar
		}
	}`)
	Nil(t, err)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		items := i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx]
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
}

func TestQueryParserFragmentSpreadDirectives(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
		baz {
			foo
			...fooBar@a@b
			... barFoo @a
			bar
		}
	}`)
	Nil(t, err)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		items := i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx]

		spread1 := items[1].fragmentSpread
		spread2 := items[2].fragmentSpread

		Equal(t, 2, len(spread1.directives))
		Equal(t, 1, len(spread2.directives))
	}
}

func TestQueryParserInlineFragment(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
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
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		items := i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx]

		Equal(t, 4, len(items))

		Equal(t, "InlineFragment", items[1].selectionType)
		Equal(t, "InlineFragment", items[2].selectionType)

		frag1 := items[1].inlineFragment
		frag2 := items[2].inlineFragment

		NotNil(t, frag1)
		NotNil(t, frag2)

		Equal(t, "", frag1.onTypeConditionName)
		Equal(t, "User", frag2.onTypeConditionName)
	}
}

func TestQueryParserInlineFragmentWithDirectives(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
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
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		items := i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx]

		frag1 := items[0].inlineFragment
		frag2 := items[1].inlineFragment

		Equal(t, 2, len(frag1.directives))
		Equal(t, 1, len(frag2.directives))
	}
}

func TestQueryParserFieldDirective(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
		client {
			foo
			bar @this_is_a_directive
			bas
			baz
		}
	}`)
	Nil(t, err)

	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		directives := i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx][1].field.directives

		NotNil(t, directives)
		Equal(t, 1, len(directives))

		_, ok := directives["this_is_a_directive"]
		True(t, ok)
	}
}

func TestQueryParserFieldInvalidDirective(t *testing.T) {
	_, _, _, err := parseQuery(`{
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
	i, fragments, operators, err := parseQuery(`{
		client {
			foo
			bar @a @b@c
			bas
			baz
		}
	}`)
	Nil(t, err)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		directives := i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx][1].field.directives

		NotNil(t, directives)
		Equal(t, 3, len(directives), "Not all directives")

		expect := []string{"a", "b", "c"}
		for _, item := range expect {
			_, ok := directives[item]
			True(t, ok, "Missing directive: "+item)
		}
	}
}

func TestQueryParserFieldWithArguments(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
		client {
			foo
			bar(a: 1,b:true c : false , d: [1,2 3 , 4,], e: $foo_bar, f: null, g: SomeEnumValue, h: {a: 1, b: true})
			baz()
		}
	}`)
	Nil(t, err)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		arguments := i.arguments[i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx][1].field.argumentsIdx]

		NotNil(t, arguments, "arguments should be defined")

		a := arguments[0]
		Equal(t, "a", a.qlFieldName)
		Equal(t, reflect.Int, a.valType)
		Equal(t, 1, a.intValue)

		b := arguments[1]
		Equal(t, "b", b.qlFieldName)
		Equal(t, reflect.Bool, b.valType)
		True(t, b.booleanValue)

		c := arguments[2]
		Equal(t, "c", c.qlFieldName)
		Equal(t, reflect.Bool, c.valType)
		False(t, c.booleanValue)

		d := arguments[3]
		Equal(t, "d", d.qlFieldName)
		Equal(t, reflect.Array, d.valType)
		list := d.listValue
		Equal(t, 4, len(list))

		for i, item := range list {
			Equal(t, reflect.Int, item.valType)
			Equal(t, i+1, item.intValue)
		}
	}
}

func TestQueryParserFieldDirectiveWithArguments(t *testing.T) {
	i, fragments, operators, err := parseQuery(`{
		client {
			foo
			bar @a(a: 1,b:true c : false) @b(a: 1,b:true c : false)@c(a: 1,b:true c : false)
			bas
			baz
		}
	}`)
	Nil(t, err)
	Equal(t, 0, len(fragments))
	Equal(t, 1, len(operators))

	for _, operator := range operators {
		directives := i.selections[i.selections[operator.selectionIdx][0].field.selectionIdx][1].field.directives

		NotNil(t, directives)
		Equal(t, 3, len(directives), "Not all directives")

		expect := []string{"a", "b", "c"}
		for _, item := range expect {
			directive, ok := directives[item]
			True(t, ok, "directive: "+item)
			arguments := i.arguments[directive.argumentsIdx]

			NotNil(t, arguments, "arguments should be defined")

			a := arguments[0]
			Equal(t, "a", a.qlFieldName)
			Equal(t, reflect.Int, a.valType, "directive: "+item)
			Equal(t, 1, a.intValue, "directive: "+item)

			b := arguments[1]
			Equal(t, "b", b.qlFieldName)
			Equal(t, reflect.Bool, b.valType, "directive: "+item)
			True(t, b.booleanValue, "directive: "+item)

			c := arguments[2]
			Equal(t, "c", c.qlFieldName)
			Equal(t, reflect.Bool, c.valType, "directive: "+item)
			False(t, c.booleanValue, "directive: "+item)
		}
	}
}

// tests if parser doesn't panic nor hangs on wired inputs
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
	_, fragments, operators, err := parseQuery(`fragment a on User {}`)
	Nil(t, err)
	Equal(t, 1, len(fragments))
	Equal(t, 0, len(operators))

	for _, fragment := range fragments {
		NotNil(t, fragment.fragment)
		Equal(t, "a", fragment.name)
		Equal(t, "User", fragment.fragment.onTypeConditionName)

	}

	_, fragments, operators, err = parseQuery(`fragment a on User {
		a
		b
	}`)
	Nil(t, err)
	Equal(t, 1, len(fragments))
	Equal(t, 0, len(operators))
}

func TestQueryParserQueryWithFragment(t *testing.T) {
	_, fragments, operators, err := parseQuery(`
		query QueryThoseHumans {}

		fragment Human on Character {
			name
			appearsIn
			friends {
				name
			}
		}
	`)
	Nil(t, err)
	Equal(t, 1, len(operators))
	Equal(t, 1, len(fragments))

	_, ok := operators["QueryThoseHumans"]
	True(t, ok)
	_, ok = fragments["Human"]
	True(t, ok)
}

func TestQueryParserUnnamed(t *testing.T) {
	_, fragments, operators, err := parseQuery(`
		query {}
		query {}
		query {}
		mutation {}
		subscription {}
	`)

	Nil(t, err)
	Equal(t, 0, len(fragments))
	Equal(t, 5, len(operators))

	exists := func(name string) {
		_, ok := operators[name]
		True(t, ok, name)
	}

	exists("unknown_query_1")
	exists("unknown_query_2")
	exists("unknown_query_3")
	exists("unknown_mutation_1")
	exists("unknown_subscription_1")
}

func TestQueryParserReportsErrors(t *testing.T) {
	// Invalid query
	_, fragments, operators, err := parseQuery(`this is not a query and should fail`)
	NotNil(t, fragments)
	NotNil(t, operators)
	NotNil(t, err)
	Equal(t, 0, len(operators))
	Equal(t, 0, len(fragments))

	// Multiple items with same name
	_, fragments, operators, err = parseQuery(`
		query foo {}
		query foo {}

		mutation bar {}
		subscription bar {}

		fragment baz on Character {}
		fragment baz on Character {}
	`)
	NotNil(t, err)
	Equal(t, 2, len(operators))
	Equal(t, 1, len(fragments))
}

func TestQueryParserMatches(t *testing.T) {
	cases := []struct {
		data    string
		oneOf   []string
		matches string
	}{
		{"foo", []string{"foo"}, "foo"},
		{"foo", []string{"bar", "foo", "baz"}, "foo"},
		{"foo", []string{"bar"}, ""},
		{"foo", []string{" bar", " foo", " baz"}, ""},
		{"foo", []string{"bar ", "foo ", "baz "}, ""},
		{"a longer string than one of the items", []string{"bar", "foo", "baz"}, ""},
		{"a longer string than one of the items", []string{"than"}, ""},
	}

	for _, case_ := range cases {
		i := newIter(true)
		i.data = case_.data
		Equal(t, case_.matches, i.matches(case_.matches), case_)
	}
}

package graphql

import (
	"strconv"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestQueryParserEmptyQuery(t *testing.T) {
	res, err := ParseQuery(``)
	Nil(t, res)
	Nil(t, err)

	res, err = ParseQuery(`  `)
	Nil(t, res)
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
		res, err := ParseQuery(option.query)
		if option.shouldFail {
			Nil(t, res, option.query)
			NotNil(t, err, option.query)
		} else {
			NotNil(t, res, option.query)
			Nil(t, err, option.query)
			Equal(t, option.expectedOperationType, res.operationType, option.query)
			Equal(t, "", res.name, option.query)
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
		res, err := ParseQuery(option.query)
		NotNil(t, res, option.query)
		Nil(t, err, option.query)
		Equal(t, option.expectedOperationType, res.operationType, option.query)
		Equal(t, "name_here", res.name, option.query)
	}
}

func TestQueryParsingQueryDirectives(t *testing.T) {
	res, err := ParseQuery(`query foo @bar {}`)
	NotNil(t, res)
	Nil(t, err)

	res, err = ParseQuery(`query @bar {}`)
	NotNil(t, res)
	Nil(t, err)
}

func TestQueryParsingQuery(t *testing.T) {
	res, err := ParseQuery(`query ($a: String) {}`)
	NotNil(t, res)
	Nil(t, err)

	res, err = ParseQuery(`query ($a: [String]) {}`)
	NotNil(t, res)
	Nil(t, err)

	res, err = ParseQuery(`query query_name( $a : String $b:Boolean) {}`)
	NotNil(t, res)
	Nil(t, err)
	Equal(t, 2, len(res.variableDefinitions))
	item1 := res.variableDefinitions[0]
	item2 := res.variableDefinitions[1]
	Equal(t, "String", item1.varType.name)
	Equal(t, "Boolean", item2.varType.name)
	Nil(t, item1.defaultValue)
	Nil(t, item2.defaultValue)

	res, err = ParseQuery(`query ($a: Boolean = true) {}`)
	NotNil(t, res)
	Nil(t, err)
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

		res, err := ParseQuery(`query ($b: ` + name + ` = ` + option.input + `) {}`)
		NotNil(t, res, option.input)
		Nil(t, err, option.input)

		item := res.variableDefinitions[0]

		if option.isInt {
			Equal(t, "IntValue", item.defaultValue.valType, option.input)
			n, _ := strconv.Atoi(option.input)
			Equal(t, n, item.defaultValue.intValue, option.input)
		} else {
			Equal(t, "FloatValue", item.defaultValue.valType, option.input)
			f, _ := strconv.ParseFloat(option.input, 64)
			Equal(t, f, item.defaultValue.floatValue, option.input)
		}
	}
}

func TestQueryParserSimpleInvalid(t *testing.T) {
	res, err := ParseQuery(`This should not get parsed`)
	Nil(t, res)
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
		res, err := ParseQuery(option)
		NotNil(t, res, option)
		Nil(t, err, option)

		Equal(t, 3, len(res.selection), "Should have 3 properties")

		selectionMap := map[string]Field{}
		for _, item := range res.selection {
			Equal(t, "Field", item.selectionType)
			NotNil(t, item.field)
			selectionMap[item.field.name] = *item.field
		}

		Contains(t, selectionMap, "a")
		Contains(t, selectionMap, "b")
		Contains(t, selectionMap, "d")
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
		res, err := ParseQuery(option)
		Nil(t, res, option)
		NotNil(t, err, option)
	}
}

func TestQueryParserSelectionInSelection(t *testing.T) {
	res, err := ParseQuery(`{
		baz {
			foo
			bar
		}
	}`)
	Nil(t, err)
	NotNil(t, res)

	NotEmpty(t, res.selection)
	selection := res.selection[0]
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
	res, err := ParseQuery(`{
		baz {
			foo
			...fooBar
			... barFoo
			bar
		}
	}`)
	Nil(t, err)
	NotNil(t, res)

	items := res.selection[0].field.selection
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
	res, err := ParseQuery(`{
		baz {
			foo
			...fooBar@a@b
			... barFoo @a
			bar
		}
	}`)
	Nil(t, err)
	NotNil(t, res)

	items := res.selection[0].field.selection

	spread1 := items[1].fragmentSpread
	spread2 := items[2].fragmentSpread

	Equal(t, 2, len(spread1.directives))
	Equal(t, 1, len(spread2.directives))
}

func TestQueryParserInlineFragment(t *testing.T) {
	res, err := ParseQuery(`{
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
	NotNil(t, res)

	items := res.selection[0].field.selection
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
	res, err := ParseQuery(`{
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
	NotNil(t, res)

	items := res.selection[0].field.selection

	frag1 := items[0].inlineFragment
	frag2 := items[1].inlineFragment

	Equal(t, 2, len(frag1.directives))
	Equal(t, 1, len(frag2.directives))
}

func TestQueryParserFieldDirective(t *testing.T) {
	res, err := ParseQuery(`{
		client {
			foo
			bar @this_is_a_directive
			bas
			baz
		}
	}`)
	Nil(t, err)
	NotNil(t, res)

	directives := res.selection[0].field.selection[1].field.directives
	NotNil(t, directives)
	Equal(t, 1, len(directives))

	_, ok := directives["this_is_a_directive"]
	True(t, ok)
}

func TestQueryParserFieldInvalidDirective(t *testing.T) {
	res, err := ParseQuery(`{
		client {
			foo
			bar @
			bas
			baz
		}
	}`)
	NotNil(t, err)
	Nil(t, res)
}

func TestQueryParserFieldMultipleDirective(t *testing.T) {
	res, err := ParseQuery(`{
		client {
			foo
			bar @a @b@c
			bas
			baz
		}
	}`)
	Nil(t, err)
	NotNil(t, res)

	directives := res.selection[0].field.selection[1].field.directives
	NotNil(t, directives)
	Equal(t, 3, len(directives), "Not all directives")

	expect := []string{"a", "b", "c"}
	for _, item := range expect {
		_, ok := directives[item]
		True(t, ok, "Missing directive: "+item)
	}
}

func TestQueryParserFieldWithArguments(t *testing.T) {
	res, err := ParseQuery(`{
		client {
			foo
			bar(a: 1,b:true c : false , d: [1,2 3 , 4,], e: $foo_bar, f: null, g: SomeEnumValue, h: {a: 1, b: true})
			baz
		}
	}`)
	Nil(t, err)
	NotNil(t, res)

	arguments := res.selection[0].field.selection[1].field.arguments
	NotNil(t, arguments, "arguments should be defined")

	a, ok := arguments["a"]
	True(t, ok)
	Equal(t, "IntValue", a.valType)
	Equal(t, 1, a.intValue)

	b, ok := arguments["b"]
	True(t, ok)
	Equal(t, "BooleanValue", b.valType)
	True(t, b.booleanValue)

	c, ok := arguments["c"]
	True(t, ok)
	Equal(t, "BooleanValue", c.valType)
	False(t, c.booleanValue)

	d, ok := arguments["d"]
	True(t, ok)
	Equal(t, "ListValue", d.valType)
	list := d.listValue
	Equal(t, 4, len(list))

	for i, item := range list {
		Equal(t, "IntValue", item.valType)
		Equal(t, i+1, item.intValue)
	}
}

func TestQueryParserFieldDirectiveWithArguments(t *testing.T) {
	res, err := ParseQuery(`{
		client {
			foo
			bar @a(a: 1,b:true c : false) @b(a: 1,b:true c : false)@c(a: 1,b:true c : false)
			bas
			baz
		}
	}`)
	Nil(t, err)
	NotNil(t, res)

	directives := res.selection[0].field.selection[1].field.directives
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
		Equal(t, "IntValue", a.valType, "directive: "+item)
		Equal(t, 1, a.intValue, "directive: "+item)

		b, ok := arguments["b"]
		True(t, ok, "directive: "+item)
		Equal(t, "BooleanValue", b.valType, "directive: "+item)
		True(t, b.booleanValue, "directive: "+item)

		c, ok := arguments["c"]
		True(t, ok, "directive: "+item)
		Equal(t, "BooleanValue", c.valType, "directive: "+item)
		False(t, c.booleanValue, "directive: "+item)
	}
}

func TestQueryParserCodeInjection(t *testing.T) {
	// test if:
	// - parser doesn't panic on wired inputs
	// - parser doesn't hang on certain inputs

	baseQuery := `query client($foo_bar: [Int!]! = 3) @directive_name(a: {a: 1, b: true}) {
		foo
		bar @a @b@c(a: 1,b:true d: [1,2 3 , 4, -11.22e+33], e: $foo_bar, f: null, g: SomeEnumValue, h: {a: 1, b: true}) {
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

	for i := range baseQuery {
		charsToInject := []string{"", "_", "-", "0", ";", " ", "#", " - ", "[", "]", "{", "}", "(", ")", ".", "e"}

		tilIndex := baseQuery[:i]
		formIndex := baseQuery[i:]

		ParseQuery(formIndex)

		for _, toInject := range charsToInject {
			// Inject extra text
			ParseQuery(tilIndex + toInject + formIndex)

			// Replace char
			ParseQuery(tilIndex + toInject + baseQuery[i+1:])
		}

		for _, toInject := range charsToInject {
			ParseQuery(tilIndex + toInject)
		}
	}
}

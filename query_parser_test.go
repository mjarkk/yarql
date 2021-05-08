package graphql

import (
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

	res, err = ParseQuery(`query ( $a : String $b:Boolean) {}`)
	NotNil(t, res)
	Nil(t, err)
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

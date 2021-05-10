package graphql

import (
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestParseQueryAndCheckNamesSimple(t *testing.T) {
	fragments, operators, errs := ParseQueryAndCheckNames(`{}`)
	NotNil(t, fragments)
	NotNil(t, operators)
	NotNil(t, errs)
	Equal(t, 0, len(errs))
	Equal(t, 1, len(operators))
	Equal(t, 0, len(fragments))
}

func TestParseQueryAndCheckNamesWithFragment(t *testing.T) {
	fragments, operators, errs := ParseQueryAndCheckNames(`
		query QueryThoseHumans {}

		fragment Human on Character {
			name
			appearsIn
			friends {
				name
			}
		} 
	`)
	Equal(t, 0, len(errs))
	Equal(t, 1, len(operators))
	Equal(t, 1, len(fragments))

	_, ok := operators["QueryThoseHumans"]
	True(t, ok)
	_, ok = fragments["Human"]
	True(t, ok)
}

func TestParseQueryAndCheckNamesUnnamed(t *testing.T) {
	fragments, operators, errs := ParseQueryAndCheckNames(`
		query {}
		query {}
		query {}
		mutation {}
		subscription {}
	`)

	Equal(t, 0, len(errs))
	Equal(t, 0, len(fragments))
	Equal(t, 5, len(operators))

	exsists := func(name string) {
		_, ok := operators[name]
		True(t, ok, name)
	}

	exsists("unknown_query_1")
	exsists("unknown_query_2")
	exsists("unknown_query_3")
	exsists("unknown_mutation_1")
	exsists("unknown_subscription_1")
}

func TestParseQueryAndCheckNamesReportErrors(t *testing.T) {
	// Invalid query
	fragments, operators, errs := ParseQueryAndCheckNames(`this is not a query and should fail`)
	NotNil(t, fragments)
	NotNil(t, operators)
	NotNil(t, errs)
	Equal(t, 1, len(errs))
	Equal(t, 0, len(operators))
	Equal(t, 0, len(fragments))

	// No operator defined in query
	fragments, operators, errs = ParseQueryAndCheckNames(``)
	Equal(t, 1, len(errs))
	Equal(t, 0, len(operators))
	Equal(t, 0, len(fragments))

	// Multiple items with same name
	fragments, operators, errs = ParseQueryAndCheckNames(`
		query foo {}
		query foo {}
		
		mutation bar {}
		subscription bar {}

		fragment baz on Character {}
		fragment baz on Character {}
	`)
	Equal(t, 3, len(errs))
	Equal(t, 2, len(operators))
	Equal(t, 1, len(fragments))
}

package tester

import (
	"testing"

	"github.com/mjarkk/go-graphql"
	"github.com/stretchr/testify/assert"
)

type TesterQuerySchema struct {
	Foo FooSchemaType
	Bar BarSchemaType
}

type FooSchemaType struct {
	ExampleField string
}

type BarSchemaType struct {
	ExampleField string
}

type TesterMutationSchema struct{}

func TestTester(t *testing.T) {
	s := graphql.NewSchema()
	err := s.Parse(TesterQuerySchema{}, TesterMutationSchema{}, nil)
	assert.NoError(t, err)

	t.Run("HasType", func(t *testing.T) {
		hasType := HasType(s, "FooSchemaType")
		assert.True(t, hasType)

		hasType = HasType(s, "BarSchemaType")
		assert.True(t, hasType)

		hasType = HasType(s, "NonExistentType")
		assert.False(t, hasType)
	})
}

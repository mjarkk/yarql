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

	t.Run("TypeKind", func(t *testing.T) {
		typeKind := TypeKind(s, "FooSchemaType")
		assert.Equal(t, "OBJECT", typeKind)

		typeKind = TypeKind(s, "BarSchemaType")
		assert.Equal(t, "OBJECT", typeKind)

		typeKind = TypeKind(s, "NonExistentType")
		assert.Equal(t, "", typeKind)
	})

	t.Run("HasFields", func(t *testing.T) {
		err := HasFields(s, "FooSchemaType", []string{"exampleField"})
		assert.NoError(t, err)

		err = HasFields(s, "FooSchemaType", []string{})
		assert.NoError(t, err)

		err = HasFields(s, "FooSchemaType", []string{"this_field_does_not_exsist"})
		assert.Error(t, err)

		err = HasFields(s, "NonExistentType", []string{"exampleField"})
		assert.Error(t, err)
	})

	t.Run("OnlyHasFields", func(t *testing.T) {
		err := OnlyHasFields(s, "FooSchemaType", []string{"exampleField"})
		assert.NoError(t, err)

		err = OnlyHasFields(s, "FooSchemaType", []string{})
		assert.Error(t, err)

		err = OnlyHasFields(s, "FooSchemaType", []string{"this_field_does_not_exsist"})
		assert.Error(t, err)
	})
}

package graphql

import (
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestQueryParserEmptyQuery(t *testing.T) {
	res, err := ParseQuery(``)
	NotNil(t, res)
	Nil(t, err)
}

func TestQueryParserEmptyBracesQuery(t *testing.T) {
	options := []struct {
		query                 string
		expectedOperationType string
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
	}

	for _, option := range options {
		res, err := ParseQuery(option.query)
		NotNil(t, res, option.query)
		Nil(t, err, option.query)
		Equal(t, option.expectedOperationType, res.operationType, option.query)
		Equal(t, "", res.name, option.query)
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
			c
		}`,
		`{
			a,
			b,
			c,
		}`,
		`{a b c}`,
		`{a,b,c}`,
	}

	for _, option := range options {
		res, err := ParseQuery(option)
		NotNil(t, res, option)
		Nil(t, err, option)
	}
}

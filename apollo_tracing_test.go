package graphql

import (
	"encoding/json"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestApolloTracing(t *testing.T) {
	s := NewSchema()
	err := s.Parse(TestExecSchemaRequestWithFieldsData{A: TestExecSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	NoError(t, err)

	query := `{a{bar}}`

	data, errs := s.Resolve(query, ResolveOptions{Tracing: false})
	for _, err := range errs {
		panic(err)
	}

	type ExtensionsFromData struct {
		Extensions struct {
			Tracing *tracer `json:"tracing"`
		} `json:"extensions"`
	}
	parsedRes := ExtensionsFromData{}
	json.Unmarshal(data, &parsedRes)

	Nil(t, parsedRes.Extensions.Tracing)

	data, errs = s.Resolve(query, ResolveOptions{Tracing: true})
	for _, err := range errs {
		panic(err)
	}

	json.Unmarshal(data, &parsedRes)
	tracing := parsedRes.Extensions.Tracing
	NotNil(t, tracing)

	Equal(t, uint8(1), tracing.Version)
	NotEqual(t, "", tracing.StartTime)
	NotEqual(t, "", tracing.EndTime)
	Greater(t, len(tracing.Execution.Resolvers), 0)
	for _, item := range tracing.Execution.Resolvers {
		True(t, json.Valid(item.Path))
		NotEqual(t, "", item.ParentType)
		NotEqual(t, "", item.FieldName)
		NotEqual(t, "", item.ReturnType)
	}
}

package graphql

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestApolloTracing(t *testing.T) {
	s, err := ParseSchema(TestExecSchemaRequestWithFieldsData{A: TestExecSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	NoError(t, err)

	query := `{a{bar}}`

	_, extensions, errs := s.Resolve(query, ResolveOptions{Tracing: false})
	for _, err := range errs {
		panic(err)
	}
	_, ok := extensions["tracing"]
	False(t, ok)

	out, extensions, errs := s.Resolve(query, ResolveOptions{Tracing: true})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"a":{"bar":"baz"}}`, out)
	tracingIntf, ok := extensions["tracing"]
	True(t, ok, "tracing entity should exists")

	tracing, ok := tracingIntf.(tracer)
	True(t, ok, "tracing entity should be of type *tracer")

	Equal(t, uint8(1), tracing.Version)
	NotEqual(t, "", tracing.StartTime)
	NotEqual(t, "", tracing.EndTime)
	fmt.Printf("%+v\n", len(tracing.Execution.Resolvers))
	for _, item := range tracing.Execution.Resolvers {
		True(t, json.Valid(item.Path))
		NotEqual(t, "", item.ParentType)
		NotEqual(t, "", item.FieldName)
		NotEqual(t, "", item.ReturnType)
	}
}

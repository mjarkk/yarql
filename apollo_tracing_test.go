package graphql

import (
	"encoding/json"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestApolloTracing(t *testing.T) {
	s, err := ParseSchema(TestExecSchemaRequestWithFieldsData{A: TestExecSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	NoError(t, err)
	_, extensions, errs := s.Resolve(`{a{bar}}`, ResolveOptions{Tracing: false})
	for _, err := range errs {
		panic(err)
	}
	_, ok := extensions["tracing"]
	False(t, ok)

	_, extensions, errs = s.Resolve(`{a{bar}}`, ResolveOptions{Tracing: true})
	for _, err := range errs {
		panic(err)
	}

	tracingIntf, ok := extensions["tracing"]
	True(t, ok, "tracing entity should exists")

	tracing, ok := tracingIntf.(*tracer)
	True(t, ok, "tracing entity should be of type *tracer")

	Equal(t, uint8(1), tracing.Version)
	NotEqual(t, "", tracing.StartTime)
	NotEqual(t, "", tracing.EndTime)
	Less(t, int64(0), tracing.Duration)
	Less(t, int64(0), tracing.Parsing.StartOffset)
	Less(t, int64(0), tracing.Parsing.Duration)
	Less(t, int64(0), tracing.Validation.StartOffset)
	Greater(t, tracing.Validation.StartOffset, tracing.Parsing.StartOffset+tracing.Parsing.Duration)
	Less(t, int64(0), tracing.Validation.Duration)
	Less(t, 0, len(tracing.Execution.Resolvers))
	for _, item := range tracing.Execution.Resolvers {
		True(t, json.Valid(item.Path))
		NotEqual(t, "", item.ParentType)
		NotEqual(t, "", item.FieldName)
		NotEqual(t, "", item.ReturnType)
		Less(t, int64(0), item.StartOffset)
		Less(t, int64(0), item.Duration)
	}
}

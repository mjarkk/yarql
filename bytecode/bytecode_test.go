package graphql

import (
	"bytes"
	"strings"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func parseQuery(query string) ([]byte, []error) {
	i := parserCtx{
		res:    []byte{},
		query:  []byte(query),
		errors: []error{},
	}
	i.parseQueryToBytecode()
	return i.res, i.errors
}

func formatHumanReadableQuery(result string) string {
	result = strings.TrimSpace(result)
	result = strings.ReplaceAll(result, " ", "")
	result = strings.ReplaceAll(result, "\t", "")
	result = strings.ReplaceAll(result, "\r", "")
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = strings.Split(line, "//")[0]
	}
	return strings.Join(lines, "\n")
}

func formatResToHumandReadable(result []byte) string {
	result = bytes.Join(bytes.Split(result, []byte{0}), []byte{'\n'})
	return strings.TrimSpace(string(result))
}

func parseQueryAndExpectResult(t *testing.T, query, expectedResult string) {
	res, errs := parseQuery(query)
	for _, err := range errs {
		panic(err.Error())
	}
	Equal(t, formatHumanReadableQuery(expectedResult), formatResToHumandReadable(res))
}

func TestParseSimpleQuery(t *testing.T) {
	parseQueryAndExpectResult(t, `{}`, `
		oq // [operator] [query]
		e  // [end of operator]
	`)
}

func TestParseSimpleQueryWrittenOut(t *testing.T) {
	parseQueryAndExpectResult(t, `query {}`, `
		oq // operator of type query
		e  // end of operator
	`)
}

func TestParseSimpleMutation(t *testing.T) {
	parseQueryAndExpectResult(t, `mutation {}`, `
		om // operator of type mutation
		e  // end of operator
	`)
}

func TestParseSimpleSubscription(t *testing.T) {
	parseQueryAndExpectResult(t, `subscription {}`, `
		os // operator of type subscription
		e  // end of operator
	`)
}

func TestParseQueryWithName(t *testing.T) {
	parseQueryAndExpectResult(t, `query banana {}`, `
		oqbanana // operator of type query with name banana
		e        // end of operator
	`)
}

func TestParseQueryWithField(t *testing.T) {
	parseQueryAndExpectResult(t, `query {
		some_field
	}`, `
		oq          // query operator
		fsome_field // field with name some_field
		e           // end field
		e           // end operator
	`)
}

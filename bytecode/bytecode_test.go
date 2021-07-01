package graphql

import (
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
		lines[i] = string(0) + strings.Split(line, "//")[0]
	}
	return result
}

func parseQueryAndExpectResult(t *testing.T, query, expectedResult string) {
	res, errs := parseQuery(query)
	Equal(t, 0, len(errs))
	Equal(t, formatHumanReadableQuery(expectedResult), string(res))
}

func TestParseSimpleQuery(t *testing.T) {
	parseQueryAndExpectResult(t, `{}`, `
		oq // [operator] [query]
		e  // [end of operator]
	`)
}

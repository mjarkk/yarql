package graphql

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func checkValidJson(t *testing.T, in string) {
	True(t, json.Valid([]byte(in)), in)
}

func TestGenerateResponse(t *testing.T) {
	checkValidJson(t, GenerateResponse("{}", nil))
}

func TestGenerateResponseWError(t *testing.T) {
	checkValidJson(t, GenerateResponse("{}", []error{errors.New("test")}))
}

func TestGenerateResponseWErrors(t *testing.T) {
	checkValidJson(t, GenerateResponse("{}", []error{errors.New("A"), errors.New("B")}))
}

func TestGenerateResponseWErrorWPath(t *testing.T) {
	j := GenerateResponse("{}", []error{ErrorWPath{err: errors.New("name invalid"), path: []string{`"users"`, `2`, `"name"`}}})
	checkValidJson(t, j)
	True(t, strings.Contains(j, `"users"`))
	True(t, strings.Contains(j, `2`))
	True(t, strings.Contains(j, `"name"`))
}

func TestGenerateResponseWErrorWLocation(t *testing.T) {
	j := GenerateResponse("{}", []error{ErrorWLocation{err: errors.New("name invalid"), line: 4, column: 9}})
	checkValidJson(t, j)
	True(t, strings.Contains(j, "4"), "Contains the line number")
	True(t, strings.Contains(j, "9"), "Contains the column number")
}

func TestHandleRequestRequestInURL(t *testing.T) {
	s, err := ParseSchema(TestExecSchemaRequestWithFieldsData{A: TestExecSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	NoError(t, err)

	res, errs := s.HandleRequest(
		"GET",
		func(key string) string {
			switch key {
			case "query":
				return "{a {bar}}"
			default:
				return ""
			}
		},
		func(key string) (string, error) { return "", errors.New("this should not be called") },
		func() []byte { return nil },
		"",
		&RequestOptions{},
	)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"data":{"a":{"bar":"baz"}}}`, res)
}

func TestHandleRequestRequestJsonBody(t *testing.T) {
	s, err := ParseSchema(TestExecSchemaRequestWithFieldsData{A: TestExecSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	NoError(t, err)

	query := `
	query Foo {
		a {
			foo
		}
	}
	query Bar {
		a {
			bar
		}
	}
	`
	query = strings.ReplaceAll(query, "\n", "\\n")
	query = strings.ReplaceAll(query, "\t", "\\t")

	res, errs := s.HandleRequest(
		"POST",
		func(key string) string { return "" },
		func(key string) (string, error) { return "", errors.New("this should not be called") },
		func() []byte {
			return []byte(`{
			"query": "` + query + `",
			"operationName": "Bar",
			"variables": {"a": "b"}
		}`)
		},
		"application/json",
		&RequestOptions{},
	)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"data":{"a":{"bar":"baz"}}}`, res)
}

func TestHandleRequestRequestForm(t *testing.T) {
	s, err := ParseSchema(TestExecSchemaRequestWithFieldsData{A: TestExecSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	NoError(t, err)

	query := `
	query Foo {
		a {
			foo
		}
	}
	query Bar {
		a {
			bar
		}
	}
	`
	query = strings.ReplaceAll(query, "\n", "\\n")
	query = strings.ReplaceAll(query, "\t", "\\t")

	res, errs := s.HandleRequest(
		"POST",
		func(key string) string { return "" },
		func(key string) (string, error) {
			switch key {
			case "operations":
				return `{
					"query": "` + query + `",
					"operationName": "Bar",
					"variables": {"a": "b"}
				}`, nil
			}
			return "", errors.New("unknown form field")
		},
		func() []byte { return nil },
		"multipart/form-data",
		&RequestOptions{},
	)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"data":{"a":{"bar":"baz"}}}`, res)
}

func TestHandleRequestRequestBatch(t *testing.T) {
	s, err := ParseSchema(TestExecSchemaRequestWithFieldsData{A: TestExecSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	NoError(t, err)

	query := `
	query Foo {
		a {
			foo
		}
	}
	query Bar {
		a {
			bar
		}
	}
	`
	query = strings.ReplaceAll(query, "\n", "\\n")
	query = strings.ReplaceAll(query, "\t", "\\t")

	res, errs := s.HandleRequest(
		"POST",
		func(key string) string { return "" },
		func(key string) (string, error) { return "", errors.New("this should not be called") },
		func() []byte {
			return []byte(`[
				{
					"query": "` + query + `",
					"operationName": "Bar",
					"variables": {"a": "b"}
				},
				{
					"query": "` + query + `",
					"operationName": "Foo",
					"variables": {"b": "c"}
				}
			]`)
		},
		"application/json",
		&RequestOptions{},
	)
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `[{"data":{"a":{"bar":"baz"}}},{"data":{"a":{"foo":null}}}]`, res)
}

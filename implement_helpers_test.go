package graphql

import (
	"errors"
	"strings"
	"testing"

	a "github.com/mjarkk/go-graphql/assert"
)

func TestHandleRequestRequestInURL(t *testing.T) {
	s := NewSchema()
	err := s.Parse(TestResolveSchemaRequestWithFieldsData{A: TestResolveSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	a.NoError(t, err)

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
	a.Equal(t, `{"data":{"a":{"bar":"baz"}}}`, string(res))
}

func TestHandleRequestRequestJsonBody(t *testing.T) {
	s := NewSchema()
	err := s.Parse(TestResolveSchemaRequestWithFieldsData{A: TestResolveSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	a.NoError(t, err)

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
	a.Equal(t, `{"data":{"a":{"bar":"baz"}}}`, string(res))
}

func TestHandleRequestRequestForm(t *testing.T) {
	s := NewSchema()
	err := s.Parse(TestResolveSchemaRequestWithFieldsData{A: TestResolveSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	a.NoError(t, err)

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
	a.Equal(t, `{"data":{"a":{"bar":"baz"}}}`, string(res))
}

func TestHandleRequestRequestBatch(t *testing.T) {
	s := NewSchema()
	err := s.Parse(TestResolveSchemaRequestWithFieldsData{A: TestResolveSchemaRequestWithFieldsDataInnerStruct{Bar: "baz"}}, M{}, nil)
	a.NoError(t, err)

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
	a.Equal(t, `[{"data":{"a":{"bar":"baz"}}},{"data":{"a":{"foo":null}}}]`, string(res))
}

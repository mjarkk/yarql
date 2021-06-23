package graphql

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/valyala/fastjson"
)

func GenerateResponse(data string, errors []error) string {
	res := `{"data":` + data
	if len(errors) > 0 {
		res += `,"errors":[`
		for i, err := range errors {
			if i > 0 {
				res += ","
			}

			ctx := ""
			errWPath, isErrWPath := err.(ErrorWPath)
			if isErrWPath && len(errWPath.path) > 0 {
				ctx = fmt.Sprintf(`,"path":[%s]`, strings.Join(errWPath.path, ","))
			}
			errWLocation, isErrWLocation := err.(ErrorWLocation)
			if isErrWLocation {
				ctx = fmt.Sprintf(`,"locations":[{"line":%d,"column":%d}]`, errWLocation.line, errWLocation.column)
			}

			res += fmt.Sprintf(`{"message":%q%s}`, err.Error(), ctx)
		}
		res += "]"
	}
	return res + "}"
}

type RequestOptions struct {
	Context     context.Context                                 // Request context can be used to verify
	Values      map[string]interface{}                          // Passed directly to the request context
	GetFormFile func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
}

func (s *Schema) HandleRequest(
	method string, // GET, POST, etc..
	getQuery func(key string) string, // URL value (needs to be un-escaped before returning)
	getFormField func(key string) (string, error), // get form field, only used if content type == form data
	getBody func() []byte, // get the request body
	contentType string, // body content type, can be an empty string if method == "GET"
	options *RequestOptions, // optional options
) (string, []error) {
	method = strings.ToUpper(method)

	query := ""
	variables := ""
	operationName := ""

	errRes := func(errorMsg string) (string, []error) {
		return "{}", []error{errors.New(errorMsg)}
	}

	if contentType == "application/json" || ((contentType == "text/plain" || contentType == "multipart/form-data") && method != "GET") {
		var body []byte
		if contentType == "multipart/form-data" {
			value, err := getFormField("operations")
			if err != nil {
				return "{}", []error{err}
			}
			body = []byte(value)

		} else {
			body = getBody()
		}
		if len(body) == 0 {
			return errRes("empty body")
		}

		var p fastjson.Parser
		v, err := p.Parse(string(body))
		if err != nil {
			return errRes("invalid json body")
		}
		if v.Type() != fastjson.TypeObject {
			return errRes("body should be a object")
		}
		jsonQuery := v.Get("query")
		if jsonQuery == nil {
			return errRes("query should be set body")
		}
		queryBytes, err := jsonQuery.StringBytes()
		if err != nil {
			return errRes("invalid query param, must be a valid string")
		}
		query = string(queryBytes)

		jsonOperationName := v.Get("operationName")
		if jsonOperationName != nil {
			t := jsonOperationName.Type()
			if t != fastjson.TypeNull {
				if t != fastjson.TypeString {
					return errRes("expected operationName to be a string but got " + t.String())
				}
				operationNameBytes, err := jsonOperationName.StringBytes()
				if err != nil {
					return errRes("invalid operationName param, must be a valid string")
				}
				operationName = string(operationNameBytes)
			}
		}

		jsonVariables := v.Get("variables")
		if jsonVariables != nil {
			t := jsonVariables.Type()
			if t != fastjson.TypeObject {
				return errRes("expected variables to be a key value object but got: " + t.String())
			}
			variables = jsonVariables.String()
		}
	} else {
		query = getQuery("query")
		variables = getQuery("variables")
		operationName = getQuery("operationName")
	}

	resolveOptions := ResolveOptions{
		OperatorTarget: operationName,
		Variables:      variables,
	}
	if options != nil {
		if options.Context != nil {
			resolveOptions.Context = options.Context
		}
		if options.Values != nil {
			resolveOptions.Values = options.Values
		}
		if options.GetFormFile != nil {
			resolveOptions.GetFormFile = options.GetFormFile
		}
	}

	return s.Resolve(query, resolveOptions)
}

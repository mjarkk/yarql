package graphql

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"strings"

	"github.com/valyala/fastjson"
)

type RequestOptions struct {
	Context     context.Context                                 // Request context can be used to verify
	Values      map[string]interface{}                          // Passed directly to the request context
	GetFormFile func(key string) (*multipart.FileHeader, error) // Get form file to support file uploading
	Tracing     bool                                            // https://github.com/apollographql/apollo-tracing
}

func (s *Schema) HandleRequest(
	method string, // GET, POST, etc..
	getQuery func(key string) string, // URL value (needs to be un-escaped before returning)
	getFormField func(key string) (string, error), // get form field, only used if content type == form data
	getBody func() []byte, // get the request body
	contentType string, // body content type, can be an empty string if method == "GET"
	options *RequestOptions, // optional options
) ([]byte, []error) {
	method = strings.ToUpper(method)

	errRes := func(errorMsg string) ([]byte, []error) {
		response := []byte(`{"data":{},"errors":[{"message":`)
		stringToJson(errorMsg, &response)
		response = append(response, []byte(`}],"extensions":{}}`)...)
		return response, []error{errors.New(errorMsg)}
	}

	if contentType == "application/json" || ((contentType == "text/plain" || contentType == "multipart/form-data") && method != "GET") {
		var body []byte
		if contentType == "multipart/form-data" {
			value, err := getFormField("operations")
			if err != nil {
				return errRes(err.Error())
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
		if v.Type() == fastjson.TypeArray {
			// Handle batch query
			responseErrs := []error{}
			response := bytes.NewBuffer([]byte("["))
			for _, item := range v.GetArray() {
				// TODO potential speed improvement by executing all items at once
				if item == nil {
					continue
				}

				if response.Len() > 1 {
					response.WriteByte(',')
				}

				query, operationName, variables, err := getBodyData(item)
				if err != nil {
					responseErrs = append(responseErrs, err)
					res, _ := errRes(err.Error())
					response.Write(res)
				} else {
					errs := s.handleSingleRequest(
						query,
						variables,
						operationName,
						options,
					)
					responseErrs = append(responseErrs, errs...)
					response.Write(s.Result)
				}
			}
			response.WriteByte(']')
			return response.Bytes(), responseErrs
		}

		query, operationName, variables, err := getBodyData(v)
		if err != nil {
			return errRes(err.Error())
		}
		errs := s.handleSingleRequest(
			query,
			variables,
			operationName,
			options,
		)
		return s.Result, errs
	}

	errs := s.handleSingleRequest(
		getQuery("query"),
		getQuery("variables"),
		getQuery("operationName"),
		options,
	)
	return s.Result, errs
}

func (s *Schema) handleSingleRequest(
	query,
	variables,
	operationName string,
	options *RequestOptions,
) []error {
	resolveOptions := ResolveOptions{
		OperatorTarget: operationName,
		Variables:      variables,
	}
	if options != nil {
		if options.Context != nil {
			resolveOptions.Context = options.Context
		}
		if options.Values != nil {
			resolveOptions.Values = &options.Values
		}
		if options.GetFormFile != nil {
			resolveOptions.GetFormFile = options.GetFormFile
		}
		resolveOptions.Tracing = options.Tracing
	}

	return s.Resolve(s2b(query), resolveOptions)
}

func getBodyData(body *fastjson.Value) (query, operationName, variables string, err error) {
	if body.Type() != fastjson.TypeObject {
		err = errors.New("body should be a object")
		return
	}

	jsonQuery := body.Get("query")
	if jsonQuery == nil {
		err = errors.New("query should be defined")
		return
	}
	queryBytes, err := jsonQuery.StringBytes()
	if err != nil {
		err = errors.New("invalid query param, must be a valid string")
		return
	}
	query = string(queryBytes)

	jsonOperationName := body.Get("operationName")
	if jsonOperationName != nil {
		t := jsonOperationName.Type()
		if t != fastjson.TypeNull {
			if t != fastjson.TypeString {
				err = errors.New("expected operationName to be a string but got " + t.String())
				return
			}
			operationNameBytes, errOut := jsonOperationName.StringBytes()
			if errOut != nil {
				err = errors.New("invalid operationName param, must be a valid string")
				return
			}
			operationName = string(operationNameBytes)
		}
	}

	jsonVariables := body.Get("variables")
	if jsonVariables != nil {
		t := jsonVariables.Type()
		if t != fastjson.TypeNull {
			if t != fastjson.TypeObject {
				err = errors.New("expected variables to be a key value object but got: " + t.String())
				return
			}
			variables = jsonVariables.String()
		}
	}

	return
}

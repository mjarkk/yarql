package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"
)

func GenerateResponse(data string, extensions map[string]interface{}, errors []error) string {
	res := bytes.NewBufferString(`{"data":`)
	res.WriteString(data)

	if len(errors) > 0 {
		res.WriteString(`,"errors":[`)
		for i, err := range errors {
			if i > 0 {
				res.WriteByte(',')
			}
			res.WriteString(`{"message":`)
			stringToJson([]byte(err.Error()), res, true)

			errWPath, isErrWPath := err.(ErrorWPath)
			if isErrWPath && len(errWPath.path) > 0 {
				res.WriteString(`,"path":`)
				errWPath.path.toJson(res)
			}
			errWLocation, isErrWLocation := err.(ErrorWLocation)
			if isErrWLocation {
				res.WriteString(`,"locations":[{"line":`)
				res.WriteString(strconv.FormatUint(uint64(errWLocation.line), 10))
				res.WriteString(`,"column":`)
				res.WriteString(strconv.FormatUint(uint64(errWLocation.column), 10))
				res.WriteString(`}]`)
			}

			res.WriteByte('}')
		}
		res.WriteByte(']')
	}
	if len(extensions) > 0 {
		extensionsJson, err := json.Marshal(extensions)
		if err == nil {
			res.WriteString(`,"extensions":`)
			res.WriteString(string(extensionsJson))
		}
	}
	res.WriteByte('}')
	return res.String()
}

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
) (string, []error) {
	method = strings.ToUpper(method)

	errRes := func(errorMsg string) (string, []error) {
		errs := []error{errors.New(errorMsg)}
		return GenerateResponse("{}", nil, errs), errs
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
		if v.Type() == fastjson.TypeArray {
			// Handle batch query
			responseErrs := []error{}
			responses := []string{}
			for _, item := range v.GetArray() {
				// TODO potential speed improvement by executing all items at once
				if item == nil {
					continue
				}

				query, operationName, variables, err := getBodyData(item)
				var res string
				var errs []error
				if err != nil {
					responseErrs = append(responseErrs, err)
					responses = append(responses, "")
				} else {
					res, errs = s.handleSingleRequest(
						query,
						variables,
						operationName,
						options,
					)
				}
				responseErrs = append(responseErrs, errs...)
				responses = append(responses, res)
			}
			return "[" + strings.Join(responses, ",") + "]", responseErrs
		}

		query, operationName, variables, err := getBodyData(v)
		if err != nil {
			return errRes(err.Error())
		}
		return s.handleSingleRequest(
			query,
			variables,
			operationName,
			options,
		)
	}

	return s.handleSingleRequest(
		getQuery("query"),
		getQuery("variables"),
		getQuery("operationName"),
		options,
	)
}

func (s *Schema) handleSingleRequest(
	query,
	variables,
	operationName string,
	options *RequestOptions,
) (string, []error) {
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
		resolveOptions.Tracing = options.Tracing
	}

	body, extensions, errs := s.Resolve(query, resolveOptions)
	return GenerateResponse(body, extensions, errs), errs
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
		if t != fastjson.TypeObject {
			err = errors.New("expected variables to be a key value object but got: " + t.String())
			return
		}
		variables = jsonVariables.String()
	}

	return
}

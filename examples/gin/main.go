package main

import (
	"io/ioutil"
	"log"
	"mime/multipart"

	"github.com/gin-gonic/gin"
	"github.com/mjarkk/go-graphql"
)

func main() {
	r := gin.Default()

	graphqlSchema, err := graphql.ParseSchema(QueryRoot{}, MethodRoot{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	r.Any("/graphql", func(c *gin.Context) {
		var form *multipart.Form

		getForm := func() (*multipart.Form, error) {
			if form != nil {
				return form, nil
			}

			var err error
			form, err = c.MultipartForm()
			return form, err
		}

		body, errors := graphqlSchema.HandleRequest(
			c.Request.Method,
			c.Query,
			func(key string) (string, error) {
				form, err := getForm()
				if err != nil {
					return "", err
				}
				values, ok := form.Value[key]
				if !ok || len(values) == 0 {
					return "", nil
				}
				return values[0], nil
			},
			func() []byte {
				requestBody, _ := ioutil.ReadAll(c.Request.Body)
				return requestBody
			},
			c.ContentType(),
			&graphql.RequestOptions{
				GetFormFile: func(key string) (*multipart.FileHeader, error) {
					form, err := getForm()
					if err != nil {
						return nil, err
					}
					files, ok := form.File[key]
					if !ok || len(files) == 0 {
						return nil, nil
					}
					return files[0], nil
				},
			},
		)
		res := graphql.GenerateResponse(body, errors)

		c.Header("Content-Type", "application/json")
		c.String(200, res)
	})

	r.Run()
}

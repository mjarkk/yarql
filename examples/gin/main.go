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

		body, errors := graphqlSchema.HandleRequest(
			c.Request.Method,
			c.Query,
			func(key string) (string, error) {
				if form != nil {
					return form.Value[key][0], nil
				}

				var err error
				form, err = c.MultipartForm()
				if err != nil {
					return "", err
				}

				return form.Value[key][0], nil
			},
			func() []byte {
				requestBody, _ := ioutil.ReadAll(c.Request.Body)
				return requestBody
			},
			c.ContentType(),
			nil,
		)
		res := graphql.GenerateResponse(body, errors)

		c.Header("Content-Type", "application/json")
		c.String(200, res)
	})

	r.Run()
}

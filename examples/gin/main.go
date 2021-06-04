package main

import (
	"io/ioutil"
	"log"

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
		requestBody, err := ioutil.ReadAll(c.Request.Body)
		errors := []error{}
		body := "{}"
		if err != nil {
			errors = append(errors, err)
		} else {
			body, errors = graphqlSchema.HandleRequest(
				c.Request.Method,
				c.Query,
				requestBody,
				c.ContentType(),
			)
		}
		res := graphql.GenerateResponse(body, errors)

		c.Header("Content-Type", "application/json")
		c.String(200, res)
	})

	r.Run()
}

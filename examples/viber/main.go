package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/mjarkk/go-graphql"
)

func main() {
	app := fiber.New()

	graphqlSchema, err := graphql.ParseSchema(QueryRoot{}, MethodRoot{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	app.All("/graphql", func(c *fiber.Ctx) error {
		body, errors := graphqlSchema.HandleRequest(
			c.Method(),
			func(key string) string { return c.Query(key) },
			func(key string) (string, error) { return c.FormValue(key), nil },
			func() []byte { return c.Body() },
			string(c.Request().Header.ContentType()),
		)
		res := graphql.GenerateResponse(body, errors)

		c.Response().Header.Set("Content-Type", "application/json")
		return c.SendString(res)
	})

	app.Listen(":3000")
}

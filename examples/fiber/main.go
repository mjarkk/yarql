package main

import (
	"log"
	"mime/multipart"

	"github.com/gofiber/fiber/v2"
	yarql "github.com/mjarkk/yarql"
)

func main() {
	app := fiber.New()

	schema := yarql.NewSchema()
	err := schema.Parse(QueryRoot{}, MethodRoot{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	app.All("/graphql", func(c *fiber.Ctx) error {
		res, _ := schema.HandleRequest(
			c.Method(),
			func(key string) string { return c.Query(key) },
			func(key string) (string, error) { return c.FormValue(key), nil },
			func() []byte { return c.Body() },
			string(c.Request().Header.ContentType()),
			&yarql.RequestOptions{
				GetFormFile: func(key string) (*multipart.FileHeader, error) { return c.FormFile(key) },
				Tracing:     true,
			},
		)

		c.Response().Header.Set("Content-Type", "application/json")
		return c.Send(res)
	})

	app.Listen(":3000")
}

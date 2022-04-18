package main

import (
	"log"
	"mime/multipart"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/mjarkk/yarql"
)

func main() {
	app := fiber.New()

	app.Use(cors.New())

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

	log.Fatal(app.Listen(":5500"))
}

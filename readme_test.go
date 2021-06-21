package graphql

import (
	"log"
	"testing"

	. "github.com/stretchr/testify/assert"
)

// Making sure the code in the readme actually works :)

type QueryRoot struct{}

type Post struct {
	Id    uint   `gq:",ID"`
	Title string `gq:"name"`
}

func (QueryRoot) ResolvePosts() []Post {
	return []Post{
		{1, "post 1"},
		{2, "post 2"},
		{3, "post 3"},
	}
}

type MethodRoot struct{}

func TestReadmeExample(t *testing.T) {
	s, err := ParseSchema(QueryRoot{}, MethodRoot{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	out := GenerateResponse(s.Resolve(`
		{
			posts {
				id
				name
			}
		}
	`, ResolveOptions{}))

	Equal(t, `{"data":{"posts":[{"id":"1","name":"post 1"},{"id":"2","name":"post 2"},{"id":"3","name":"post 3"}]}}`, out)
}

package graphql

import (
	"log"
	"testing"

	a "github.com/stretchr/testify/assert"
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
	s := NewSchema()

	err := s.Parse(QueryRoot{}, MethodRoot{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	errs := s.Resolve([]byte(`
		{
			posts {
				id
				name
			}
		}
	`), ResolveOptions{})
	for _, err := range errs {
		log.Fatal(err)
	}

	a.Equal(t, `{"data":{"posts":[{"id":"1","name":"post 1"},{"id":"2","name":"post 2"},{"id":"3","name":"post 3"}]},"errors":[],"extensions":{}}`, string(s.Result))
}

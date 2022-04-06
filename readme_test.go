package graphql

import (
	"log"
	"testing"

	a "github.com/mjarkk/yarql/assert"
)

// Making sure the code in the readme actually works :)

// QueryRoot defines the entry point for all graphql queries
type QueryRoot struct{}

// Post defines a post someone made
type Post struct {
	ID    uint   `gq:"id,ID"`
	Title string `gq:"name"`
}

// ResolvePosts returns all posts
func (QueryRoot) ResolvePosts() []Post {
	return []Post{
		{1, "post 1"},
		{2, "post 2"},
		{3, "post 3"},
	}
}

// MethodRoot defines the entry for all method graphql queries
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

	a.Equal(t, `{"data":{"posts":[{"id":"1","name":"post 1"},{"id":"2","name":"post 2"},{"id":"3","name":"post 3"}]}}`, string(s.Result))
}

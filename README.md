# WIP Grahql library for GoLang

## Example
```go
package main

import (
    "log"
    "github.com/mjarkk/go-grahql"
)

type Post struct {
	Id    uint
	Title string `gqName:"name"`
}

type QueryRoot struct{}

func (QueryRoot) ResolvePosts() []Post {
	return []Post{
		{1, "post 1"},
		{2, "post 2"},
		{3, "post 3"},
	}
}

type MethodRoot struct{}

func main() {
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
	`, ""))

    fmt.Println(out)
    // {"data": {
    //   "posts": [
    //     {"id":1,"name":"post 1"},
    //     {"id":2,"name":"post 2"},
    //     {"id":3,"name":"post 3"}
    //   ]
    // }}
}
```
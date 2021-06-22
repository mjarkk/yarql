# Graphql library for GoLang

Just a different approach to making graphql servers in Go

## Features

- Easy to use and not much code required
- Build on top of the graphql spec 2018
- No code generators
- Schema is based on code
- Only 1 dependency
- Easy to implement, see the [gin](https://github.com/mjarkk/go-graphql/blob/main/examples/gin/main.go) and [viber](https://github.com/mjarkk/go-graphql/blob/main/examples/viber/main.go) examples

## Example

See the [/examples](https://github.com/mjarkk/go-graphql/tree/main/examples) folder for more examples

```go
package main

import (
    "log"
    "github.com/mjarkk/go-graphql"
)

type Post struct {
	Id    uint `gq:",ID"`
	Title string `gq:"name"`
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
    s, err := graphql.ParseSchema(QueryRoot{}, MethodRoot{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	out := graphql.GenerateResponse(s.Resolve(`
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
    //     {"id": "1", "name": "post 1"},
    //     {"id": "2", "name": "post 2"},
    //     {"id": "3", "name": "post 3"}
    //   ]
    // }}
}
```

## Docs

### Defining a field

All fields names are by default changed to graphql names, for example `VeryNice` changes to `veryNice`. There is one exception to the rule when the second letter is also upper case like `FOO` will stay `FOO`

In a struct:

```go
struct {
	A string
}
```

A resolver function inside the a struct:

```go
struct {
	A func() string
}
```

A resolver attached to the struct.

Name Must start with `Resolver` then at least one uppercase letter

```go
type A struct {}
func (A) ResolveA() string {return "Ahh yea"}
```

### Supported input and output value types

These go data kinds should be globally accepted:

- bool
- int, int(8 | 16 | 32 | 64)
- uint, uint(8 | 16 | 32 | 64)
- float(32 | 64)
- array
- ptr
- string
- struct

### Ignore fields

```go
struct {
	// internal fields are ignored
	bar string

	// ignore public fields
	Bar string `gq:"-"`
}
```

### Custom field name

```go
struct {
	// Change the graphql field name to "bar"
	Foo string `gq:"bar"`
}
```

### Label as ID field

```go
struct {
	// Notice the "," before the id
	Id string `gq:",id"`

	// Pointers and numbers are also supported
	// NOTE NUMBERS WILL BE CONVERTED TO STRINGS IN OUTPUT
	PostId *int `gq:",id"`
}
```

### Methods and field arguments

Add a struct to the arguments of a resolver or func field to define arguments

```go
func (A) ResolveUserID(args struct{ Id int }) int {
	return args.Id
}
```

### Resolver error response

You can add an error response argument to send back potential errors.

These errors will appear in the errors array of the response.

```go
func (A) ResolveMe() (*User, error) {
	me, err := fetchMe()
	return me, err
}
```

### Context

You can add `*graphql.Ctx` to every resolver of func field to get more information about the request or user set properties

```go
func (A) ResolveMe(ctx *graphql.Ctx) User {
	return ctx.Values["me"].(User)
}
```

### Optional fields

All types that might be `nil` will be optional fields, by default these fields are:

- Pointers
- Arrays

## Alternatives

- [graph-gophers/graphql-go](https://github.com/graph-gophers/graphql-go)
- [ccbrown/api-fu](https://github.com/ccbrown/api-fu)
- [99designs/gqlgen](https://github.com/99designs/gqlgen)
- [graphql-go/graphql](https://github.com/graphql-go/graphql)

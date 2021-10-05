# Graphql library for GoLang

Just a different approach to making graphql servers in Go

## Features

- Easy to use and not much code required
- Schema based on code
- Build on top of the graphql spec 2018
- No code generators
- Only 2 dependencies
- Easy to implement in many webservers, see the [gin](https://github.com/mjarkk/go-graphql/blob/main/examples/gin/main.go) and [viber](https://github.com/mjarkk/go-graphql/blob/main/examples/viber/main.go) examples
- File upload support
- Supports [Apollo tracing](https://github.com/apollographql/apollo-tracing)

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
	s := NewSchema()

    err := s.Parse(QueryRoot{}, MethodRoot{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	errs := s.Resolve(`
		{
			posts {
				id
				name
			}
		}
	`, "")
	for _, err := range errs {
		log.Fatal(err)
	}

    fmt.Println(string(s.Result))
    // {"data": {
    //   "posts": [
    //     {"id": "1", "name": "post 1"},
    //     {"id": "2", "name": "post 2"},
    //     {"id": "3", "name": "post 3"}
    //   ]
    // },"errors":[],"extensions":{}}
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

Name Must start with `Resolver` followed by one uppercase letter

_The resolve identifier is trimmed away in the graphql name_

```go
type A struct {}
func (A) ResolveA() string {return "Ahh yea"}
```

### Supported input and output value types

These go data kinds should be globally accepted:

- `bool`
- `int` _all bit sizes_
- `uint` _all bit sizes_
- `float` _all bit sizes_
- `array`
- `ptr`
- `string`
- `struct`

There are also special values:

- `time.Time` _converted from/to ISO 8601_
- `*multipart.FileHeader` _get file from multipart form_

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

### Enums

Enums can be defined like so

Side note on using enums as argument, It might return a nullish value if the user didn't provide a value

```go
// The enum type, everywhere where this value is used it will be converted to an enum in graphql
// This can also be a: string, int(*) or uint(*)
type Fruit uint8

const (
	Apple Fruit = iota
	Peer
	Grapefruit
)


func main() {
	s := NewSchema()

	// The map key is the enum it's key in graphql
	// The map value is the go value the enum key is mapped to or the other way around
	// Also the .RegisterEnum(..) method must be called before .Parse(..)
	s.RegisterEnum(map[string]Fruit{
		"APPLE":      Apple,
		"PEER":       Peer,
		"GRAPEFRUIT": Grapefruit,
	})

	s.Parse(QueryRoot{}, MethodRoot{}, nil)
}
```

### Directives

These directives are added by default:

- `@include(if: Boolean!)` _on Fields and fragments_
- `@skip(if: Boolean!)` _on Fields and fragments_

To add custom directives:

```go
func main() {
	s := NewSchema()

	// Also the .RegisterEnum(..) method must be called before .Parse(..)
	s.RegisterDirective(Directive{
		// What is the name of the directive
		Name: "skip_2",

		// Where can this directive be used in the query
		Where: []DirectiveLocation{
			DirectiveLocationField,
			DirectiveLocationFragment,
			DirectiveLocationFragmentInline,
		},

		// This methods's input work equal to field arguments
		// tough the output is required to return DirectiveModifier
		// This method is called always when the directive is used
		Method: func(args struct{ If bool }) DirectiveModifier {
			return DirectiveModifier{
				Skip: args.If,
			}
		},

		// The description of the directive
		Description: "Directs the executor to skip this field or fragment when the `if` argument is true.",
	})

	s.Parse(QueryRoot{}, MethodRoot{}, nil)
}
```

### File upload

_NOTE: This is NOT [graphql-multipart-request-spec](https://github.com/jaydenseric/graphql-multipart-request-spec) tough this is based on [graphql-multipart-request-spec #55](https://github.com/jaydenseric/graphql-multipart-request-spec/issues/55)_

In your go code add `*multipart.FileHeader` to a methods inputs

```go
func (SomeStruct) ResolveUploadFile(args struct{ File *multipart.FileHeader }) string {
	// ...
}
```

In your graphql query you can now do:

```gql
  uploadFile(file: "form_file_field_name")
```

In your request add a form file with the field name: `form_file_field_name`

## Performance

Below shows a benchmark of fetching the graphql schema (query parsing + data fetching)

_Note: This benchmark also profiles the cpu and that effects the score by a bit_

```sh
# go test -benchmem -bench "^(BenchmarkResolve)\$"
# goos: darwin
# cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkResolve-12    	   13246	     83731 ns/op	    1344 B/op	      47 allocs/op
```

## Alternatives

- [graph-gophers/graphql-go](https://github.com/graph-gophers/graphql-go)
- [ccbrown/api-fu](https://github.com/ccbrown/api-fu)
- [99designs/gqlgen](https://github.com/99designs/gqlgen)
- [graphql-go/graphql](https://github.com/graphql-go/graphql)

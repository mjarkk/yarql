package main

// QueryRoot defines the entry point for all graphql queries
type QueryRoot struct{}

// MethodRoot defines the entry for all method graphql queries
type MethodRoot struct{}

// User contains information about a user
type User struct {
	ID    uint `gq:"id"`
	Name  string
	Email string
}

// Post contains information about a post
type Post struct {
	Title string
}

// ResolveUsers resolves all users
func (QueryRoot) ResolveUsers() []User {
	return []User{
		{ID: 1, Name: "Pieter", Email: "pietpaulesma@gmail.com"},
		{ID: 2, Name: "Peer", Email: "peer@gmail.com"},
		{ID: 3, Name: "Henk", Email: "henk@gmail.com"},
	}
}

// ResolvePosts resolves the posts of a specific user posts
func (u User) ResolvePosts() []Post {
	if u.ID == 1 {
		return []Post{
			{Title: "Very nice"},
			{Title: "Very cool"},
			{Title: "Ok"},
		}
	}
	return []Post{}
}

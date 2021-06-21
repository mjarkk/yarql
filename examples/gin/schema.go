package main

type QueryRoot struct{}

type MethodRoot struct{}

type User struct {
	ID    uint `gq:"id"`
	Name  string
	Email string
}

type Post struct {
	Title string
}

func (QueryRoot) ResolveUsers() []User {
	return []User{
		{ID: 1, Name: "Pieter", Email: "pietpaulesma@gmail.com"},
		{ID: 2, Name: "Peer", Email: "peer@gmail.com"},
		{ID: 3, Name: "Henk", Email: "henk@gmail.com"},
	}
}

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

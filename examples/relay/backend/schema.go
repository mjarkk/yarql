package main

import "github.com/mjarkk/go-graphql"

type QueryRoot struct{}

type MethodRoot struct{}

type Node interface {
	ResolveId() (uint, graphql.AttrIsID)
}

type User struct {
	ID    uint `gq:"-"`
	Name  string
	Email string
}

var _ = graphql.Implements((*Node)(nil), User{})

func (u User) ResolveId() (uint, graphql.AttrIsID) {
	return u.ID, 0
}

type Post struct {
	ID    uint `gq:"-"`
	Title string
}

var _ = graphql.Implements((*Node)(nil), Post{})

func (p Post) ResolveId() (uint, graphql.AttrIsID) {
	return p.ID, 0
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

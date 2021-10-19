package main

import (
	"fmt"

	"github.com/mjarkk/go-graphql"
)

type QueryRoot struct{}

type MethodRoot struct{}

type Node interface {
	ResolveId() (uint, graphql.AttrIsID)
}

type Todo struct {
	ID    uint `gq:"-"` // ignored because of (Todo).ResolveId()
	Title string
	Done  bool
}

var _ = graphql.Implements((*Node)(nil), Todo{})

// ResolveId implements the Node interface
func (u Todo) ResolveId() (uint, graphql.AttrIsID) {
	return u.ID, 0
}

var todoIdx = uint(3)

var todos = []Todo{
	{ID: 1, Title: "Get groceries", Done: false},
	{ID: 2, Title: "Make TODO app", Done: true},
}

func (QueryRoot) ResolveTodos() []Todo {
	return todos
}

type GetTodoArgs struct {
	ID uint `gq:"id,id"` // rename field to id and label field to have ID type
}

func (q QueryRoot) ResolveTodo(args GetTodoArgs) *Todo {
	for _, todo := range todos {
		if todo.ID == args.ID {
			return &todo
		}
	}
	return nil
}

type CreateTodoArgs struct {
	Title string
}

func (m MethodRoot) ResolveCreateTodo(args CreateTodoArgs) Todo {
	todo := Todo{
		ID:    todoIdx,
		Title: fmt.Sprint(args.Title), // Copy title
		Done:  false,
	}
	todos = append(todos, todo)
	todoIdx++
	return todo
}

type UpdateTodoArgs struct {
	ID    uint `gq:"id,id"` // rename field to id and label field to have ID type
	Title *string
	Done  *bool
}

func (m MethodRoot) ResolveUpdateTodo(args UpdateTodoArgs) (Todo, error) {
	idx := -1
	for i, todo := range todos {
		if todo.ID == args.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return Todo{}, fmt.Errorf("todo with id %d not found", args.ID)
	}

	todo := todos[idx]
	if args.Title != nil {
		todo.Title = *args.Title
	}
	if args.Done != nil {
		todo.Done = *args.Done
	}
	todos[idx] = todo

	return todo, nil
}

func (m MethodRoot) ResolveDeleteTodo(args GetTodoArgs) ([]Todo, error) {
	idx := -1
	for i, todo := range todos {
		if todo.ID == args.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("todo with id %d not found", args.ID)
	}

	todos = append(todos[:idx], todos[idx+1:]...)
	return todos, nil
}

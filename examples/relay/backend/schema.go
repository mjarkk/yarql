package main

import (
	"fmt"

	"github.com/mjarkk/yarql"
)

// QueryRoot defines the entry point for all graphql queries
type QueryRoot struct{}

// MethodRoot defines the entry for all method graphql queries
type MethodRoot struct{}

// Node defines the required relay Node interface
//   ref: https://relay.dev/docs/guides/graphql-server-specification/
type Node interface {
	ResolveId() (uint, yarql.AttrIsID)
}

// Todo respresents a todo entry
type Todo struct {
	ID    uint `gq:"-"` // ignored because of (Todo).ResolveId()
	Title string
	Done  bool
}

var _ = yarql.Implements((*Node)(nil), Todo{})

// ResolveId implements the Node interface
func (u Todo) ResolveId() (uint, yarql.AttrIsID) {
	return u.ID, 0
}

var todoIdx = uint(3)

var todos = []Todo{
	{ID: 1, Title: "Get groceries", Done: false},
	{ID: 2, Title: "Make TODO app", Done: true},
}

// ResolveTodos returns all todos
func (QueryRoot) ResolveTodos() []Todo {
	return todos
}

// GetTodoArgs are the arguments for the ResolveTodo
type GetTodoArgs struct {
	ID uint `gq:"id,id"` // rename field to id and label field to have ID type
}

// ResolveTodo returns a todo by id
func (q QueryRoot) ResolveTodo(args GetTodoArgs) *Todo {
	for _, todo := range todos {
		if todo.ID == args.ID {
			return &todo
		}
	}
	return nil
}

// CreateTodoArgs are the arguments for the ResolveCreateTodo
type CreateTodoArgs struct {
	Title string
}

// ResolveCreateTodo creates a new todo
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

// UpdateTodoArgs are the arguments for the ResolveUpdateTodo
type UpdateTodoArgs struct {
	ID    uint `gq:"id,id"` // rename field to id and label field to have ID type
	Title *string
	Done  *bool
}

// ResolveUpdateTodo updates a todo
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

// ResolveDeleteTodo deletes a todo
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

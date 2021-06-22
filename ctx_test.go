package graphql

import (
	"errors"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestContextPath(t *testing.T) {
	ctx := Ctx{}
	var nilArr []string
	Equal(t, nilArr, ctx.Path())
	ctx.path = &[]string{"a", "b"}
	Equal(t, []string{"a", "b"}, ctx.Path())
}

func TestContextHasErrors(t *testing.T) {
	ctx := Ctx{}
	Equal(t, false, ctx.HasErrors())
	ctx.errors = []error{}
	Equal(t, false, ctx.HasErrors())
	ctx.errors = []error{errors.New("YOU PICKED THE WRONG HOUSE FOOOOOOL")}
	Equal(t, true, ctx.HasErrors())
}

func TestContextGetErrors(t *testing.T) {
	ctx := Ctx{}
	Equal(t, []error{}, ctx.Errors())
	ctx.errors = []error{errors.New("ahh yes")}
	Equal(t, []error{errors.New("ahh yes")}, ctx.Errors())
}

func TestContextAddError(t *testing.T) {
	ctx := Ctx{}
	ctx.AddError(errors.New("ah yes"))
	errs := ctx.Errors()
	Equal(t, 1, len(errs))
	Equal(t, "ah yes", errs[0].Error())
}

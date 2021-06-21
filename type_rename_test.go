package graphql

import (
	"reflect"
	"testing"

	. "github.com/stretchr/testify/assert"
)

var _ = TypeRename(TestTypeRenameData{}, "Foo")

type TestTypeRenameData struct{}

func TestTypeRename(t *testing.T) {
	ctx := newParseCtx()
	obj, err := ctx.check(reflect.TypeOf(TestTypeRenameData{}), false)
	NoError(t, err)

	Equal(t, "Foo", obj.typeName)
	_, ok := (*ctx.types)["Foo"]
	True(t, ok)
}

func TestTypeRenameFails(t *testing.T) {
	Panics(t, func() {
		TypeRename(TestTypeRenameData{}, "")
	}, "Should panic when giving no type rename name")

	Panics(t, func() {
		TypeRename(struct{}{}, "Foo")
	}, "Should panic when giving a non global struct")

	Panics(t, func() {
		TypeRename(123, "Foo")
	}, "Should panic when giving a non struct")
}

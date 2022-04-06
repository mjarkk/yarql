package graphql

import (
	"reflect"
	"testing"

	a "github.com/mjarkk/yarql/assert"
)

var _ = TypeRename(TestTypeRenameData{}, "Foo")

type TestTypeRenameData struct{}

func TestTypeRename(t *testing.T) {
	ctx := newParseCtx()
	obj, err := ctx.check(reflect.TypeOf(TestTypeRenameData{}), false)
	a.NoError(t, err)

	a.Equal(t, "Foo", obj.typeName)
	_, ok := ctx.schema.types["Foo"]
	a.True(t, ok)
}

func TestTypeRenameFails(t *testing.T) {
	a.Panics(t, func() {
		TypeRename(TestTypeRenameData{}, "")
	}, "Should panic when giving no type rename name")

	a.Panics(t, func() {
		TypeRename(struct{}{}, "Foo")
	}, "Should panic when giving a non global struct")

	a.Panics(t, func() {
		TypeRename(123, "Foo")
	}, "Should panic when giving a non struct")
}

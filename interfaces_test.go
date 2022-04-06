package graphql

import (
	"reflect"
	"testing"

	a "github.com/mjarkk/yarql/assert"
)

type InterfaceSchema struct {
	Bar     BarWImpl
	Baz     BazWImpl
	Generic InterfaceType
}

type InterfaceType interface {
	ResolveFoo() string
	ResolveBar() string
}

type BarWImpl struct {
	ExtraBarField string
}

func (BarWImpl) ResolveFoo() string { return "this is bar" }
func (BarWImpl) ResolveBar() string { return "This is bar" }

type BazWImpl struct {
	ExtraBazField string
}

func (BazWImpl) ResolveFoo() string { return "this is baz" }
func (BazWImpl) ResolveBar() string { return "This is baz" }

func TestInterfaceType(t *testing.T) {
	implementationMapLen := len(implementationMap)
	structImplementsMapLen := len(structImplementsMap)

	Implements((*InterfaceType)(nil), BarWImpl{})
	a.Equal(t, implementationMapLen+1, len(implementationMap))
	a.Equal(t, structImplementsMapLen+1, len(structImplementsMap))

	Implements((*InterfaceType)(nil), BazWImpl{})
	a.Equal(t, implementationMapLen+1, len(implementationMap))
	a.Equal(t, structImplementsMapLen+2, len(structImplementsMap))

	_, err := newParseCtx().check(reflect.TypeOf(InterfaceSchema{}), false)
	a.Nil(t, err)
}

func TestInterfaceInvalidInput(t *testing.T) {
	a.Panics(t, func() {
		Implements(nil, BarWImpl{})
	}, "cannot use nil as interface value")

	a.Panics(t, func() {
		Implements((*InterfaceType)(nil), nil)
	}, "cannot use nil as type value")

	a.Panics(t, func() {
		Implements(struct{}{}, BarWImpl{})
	}, "cannot use non interface type as interface value")

	a.Panics(t, func() {
		Implements((*InterfaceType)(nil), "this is not a valid type")
	}, "cannot use non struct type as type value")

	a.Panics(t, func() {
		Implements((*interface{})(nil), BarWImpl{})
	}, "cannot use inline interface type as interface value")

	a.Panics(t, func() {
		Implements((*InterfaceType)(nil), struct{}{})
	}, "cannot use inline struct type as type value")

	type InvalidStruct struct{}
	a.Panics(t, func() {
		Implements((*InterfaceType)(nil), InvalidStruct{})
	}, "cannot use struct that doesn't implement the interface")
}

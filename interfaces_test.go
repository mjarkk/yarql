package graphql

import (
	"reflect"
	"testing"

	a "github.com/stretchr/testify/assert"
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

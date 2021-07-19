package graphql

import (
	"testing"

	"github.com/mjarkk/go-graphql/bytecode"
	. "github.com/stretchr/testify/assert"
)

func bytecodeParseAndTest(t *testing.T, query string, queries interface{}, methods interface{}) (string, []error) {
	s, err := ParseSchema(queries, methods, nil)
	NoError(t, err, query)

	ctx := BytecodeCtx{
		schema: s,
		query: bytecode.ParserCtx{
			Res:    []byte{},
			Query:  []byte{},
			Errors: []error{},
		},
		result: []byte{},
		charNr: 0,
	}
	bytes, errs := ctx.BytecodeResolve([]byte(query), BytecodeParseOptions{NoMeta: true})
	return string(bytes), errs
}

func TestBytecodeResolveOnlyOperation(t *testing.T) {
	_, errs := bytecodeParseAndTest(t, `{}`, TestExecEmptyQueryDataQ{}, M{})
	for _, err := range errs {
		panic(err.Error())
	}
}

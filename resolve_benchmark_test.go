package graphql

import (
	"testing"
)

func BenchmarkCheckNames(b *testing.B) {
	// BenchmarkCheckNames-12    	    2277	    480462 ns/op	  189581 B/op	    2343 allocs/op

	for i := 0; i < b.N; i++ {
		ParseQueryAndCheckNames(schemaQuery, nil)
	}
}

func BenchmarkResolve(b *testing.B) {
	// BenchmarkResolve-12    	     854	   1383447 ns/op	  833377 B/op	   11670 allocs/op // First ran
	// BenchmarkResolve-12    	     852	   1379526 ns/op	  833150 B/op	   11668 allocs/op // Placed some resolver global variables in global scope
	// BenchmarkResolve-12    	     915	   1283598 ns/op	  782547 B/op	   10384 allocs/op // Use path from Ctx
	// BenchmarkResolve-12    	     886	   1308011 ns/op	  782452 B/op	   10379 allocs/op // Use array for value

	s, _ := ParseSchema(TestExecSchemaRequestWithFieldsData{}, M{}, nil)
	for i := 0; i < b.N; i++ {
		s.Resolve(schemaQuery, ResolveOptions{})
	}
}

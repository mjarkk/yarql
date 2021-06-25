package graphql

import (
	"log"
	"os"
	"runtime"
	"runtime/pprof"
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
	// BenchmarkResolve-12    	    1202	    998317 ns/op	  313687 B/op	    6064 allocs/op // Reduced a lot of string usage
	// BenchmarkResolve-12    	    1294	    898636 ns/op	  307930 B/op	    5686 allocs/op // Change value formatting to allocate less

	s, _ := ParseSchema(TestExecSchemaRequestWithFieldsData{}, M{}, nil)
	for i := 0; i < b.N; i++ {
		s.Resolve(schemaQuery, ResolveOptions{})
	}
}

func BenchmarkResolveWithFormat(b *testing.B) {
	s, _ := ParseSchema(TestExecSchemaRequestWithFieldsData{}, M{}, nil)
	for i := 0; i < b.N; i++ {
		GenerateResponse(s.Resolve(schemaQuery, ResolveOptions{}))
	}

	f, err := os.Create("memprofile")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Fatal("could not write memory profile: ", err)
	}
}

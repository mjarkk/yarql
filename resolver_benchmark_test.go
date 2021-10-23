package graphql

import (
	"log"
	"os"
	"runtime/pprof"
	"testing"
)

func BenchmarkResolve(b *testing.B) {
	// BenchmarkResolve-12    	   10750	    102765 ns/op	    4500 B/op	      49 allocs/op

	s := NewSchema()
	s.Parse(TestResolveSchemaRequestWithFieldsData{}, M{}, nil)

	query := []byte(schemaQuery)

	opts := ResolveOptions{}

	f, err := os.Create("memprofile")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

	var errs []error
	for i := 0; i < b.N; i++ {
		errs = s.Resolve(query, opts)
		for _, err := range errs {
			panic(err)
		}
	}

	// runtime.GC()
	// if err := pprof.WriteHeapProfile(f); err != nil {
	// 	log.Fatal("could not write memory profile: ", err)
	// }
}

type HelloWorldSchema struct {
	Hello string
}

func BenchmarkHelloWorldResolve(b *testing.B) {
	s := NewSchema()
	s.Parse(HelloWorldSchema{Hello: "World"}, M{}, nil)

	query := []byte(`{hello}`)

	opts := ResolveOptions{}

	// f, err := os.Create("memprofile")
	// if err != nil {
	// 	log.Fatal("could not create memory profile: ", err)
	// }
	// defer f.Close()

	// if err := pprof.StartCPUProfile(f); err != nil {
	// 	log.Fatal("could not start CPU profile: ", err)
	// }
	// defer pprof.StopCPUProfile()

	var errs []error
	for i := 0; i < b.N; i++ {
		errs = s.Resolve(query, opts)
		for _, err := range errs {
			panic(err)
		}
	}
}

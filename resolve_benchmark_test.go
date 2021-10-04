package graphql

import (
	"log"
	"os"
	"runtime/pprof"
	"testing"
)

func BenchmarkQueryParser(b *testing.B) {
	// On laptop
	// BenchmarkQueryParser-12    	   35697	     33170 ns/op	   17536 B/op	     331 allocs/op
	// BenchmarkQueryParser-12    	   37735	     30622 ns/op	   17488 B/op	     329 allocs/op
	// BenchmarkQueryParser-12    	   35721	     30887 ns/op	   10793 B/op	     273 allocs/op
	// BenchmarkQueryParser-12    	   51865	     19334 ns/op	   10770 B/op	     273 allocs/op
	// BenchmarkQueryParser-12    	   50334	     19974 ns/op	   14362 B/op	     241 allocs/op
	// BenchmarkQueryParser-12    	  124452	      9453 ns/op	    1225 B/op	      87 allocs/op

	// On desktop
	// BenchmarkQueryParser-16    	   67248	     17757 ns/op	   10716 B/op	     172 allocs/op
	// BenchmarkQueryParser-16    	   76287	     15323 ns/op	    8130 B/op	      95 allocs/op
	// BenchmarkQueryParser-16    	   85382	     13723 ns/op	    3128 B/op	      91 allocs/op
	// BenchmarkQueryParser-16    	   88876	     13199 ns/op	    1240 B/op	      87 allocs/op

	// f, err := os.Create("memprofile")
	// if err != nil {
	// 	log.Fatal("could not create memory profile: ", err)
	// }
	// defer f.Close()

	// if err := pprof.StartCPUProfile(f); err != nil {
	// 	log.Fatal("could not start CPU profile: ", err)
	// }
	// defer pprof.StopCPUProfile()

	iter := newIter(false)
	for i := range iter.selections {
		iter.selections[i] = make(selectionSet, 5)
	}
	for i := range iter.arguments {
		iter.arguments[i] = make(arguments, 5)
	}

	for i := 0; i < b.N; i++ {
		iter.parseQuery(schemaQuery)
	}

	// runtime.GC()
	// if err := pprof.WriteHeapProfile(f); err != nil {
	// 	log.Fatal("could not write memory profile: ", err)
	// }
}

func BenchmarkBytecodeResolve(b *testing.B) {
	s := NewSchema()
	s.Parse(TestExecSchemaRequestWithFieldsData{}, M{}, nil)
	ctx := NewBytecodeCtx(s)

	query := []byte(schemaQuery)

	opts := BytecodeParseOptions{}

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
		errs = ctx.BytecodeResolve(query, opts)
		for _, err := range errs {
			panic(err)
		}
	}

	// runtime.GC()
	// if err := pprof.WriteHeapProfile(f); err != nil {
	// 	log.Fatal("could not write memory profile: ", err)
	// }
}

func BenchmarkEncodeString(b *testing.B) {
	inputString1 := "abc"
	inputString2 := "Some long string that includes spaces  and a ."
	inputString3 := `Wow this includes \\ and && and <> and ""`
	inputString4 := "The `String` scalar type represents textual data, represented as UTF-8 character sequences. The String type is most often used by GraphQL to represent free-form human-readable text."
	out := []byte{}

	for i := 0; i < b.N; i++ {
		stringToJson(inputString1, &out)
		stringToJson(inputString2, &out)
		stringToJson(inputString3, &out)
		stringToJson(inputString4, &out)
	}
}

type HelloWorldSchema struct {
	Hello string
}

func BenchmarkBytecodeHelloWorldResolve(b *testing.B) {
	s := NewSchema()
	s.Parse(HelloWorldSchema{Hello: "World"}, M{}, nil)
	ctx := NewBytecodeCtx(s)

	query := []byte(`{hello}`)

	opts := BytecodeParseOptions{}

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
		errs = ctx.BytecodeResolve(query, opts)
		for _, err := range errs {
			panic(err)
		}
	}
}

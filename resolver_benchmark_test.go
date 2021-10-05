package graphql

import (
	"log"
	"os"
	"runtime/pprof"
	"testing"
)

func BenchmarkBytecodeResolve(b *testing.B) {
	s := NewSchema()
	s.Parse(TestResolveSchemaRequestWithFieldsData{}, M{}, nil)
	ctx := NewCtx(s)

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
		errs = ctx.Resolve(query, opts)
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
	ctx := NewCtx(s)

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
		errs = ctx.Resolve(query, opts)
		for _, err := range errs {
			panic(err)
		}
	}
}

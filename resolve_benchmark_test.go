package graphql

import (
	"log"
	"os"
	"runtime/pprof"
	"testing"
	"time"
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

func BenchmarkResolve(b *testing.B) {
	// TODO add benchmark without pprof it's faster and already goes above 10.000 :)

	// On laptop
	// BenchmarkResolve-12    	     854	   1383447 ns/op	  833377 B/op	   11670 allocs/op // First ran
	// BenchmarkResolve-12    	     852	   1379526 ns/op	  833150 B/op	   11668 allocs/op
	// BenchmarkResolve-12    	     915	   1283598 ns/op	  782547 B/op	   10384 allocs/op
	// BenchmarkResolve-12    	     886	   1308011 ns/op	  782452 B/op	   10379 allocs/op
	// BenchmarkResolve-12    	    1202	    998317 ns/op	  313687 B/op	    6064 allocs/op
	// BenchmarkResolve-12    	    1294	    898636 ns/op	  307930 B/op	    5686 allocs/op
	// BenchmarkResolve-12    	    3206	    345997 ns/op	   57292 B/op	    3686 allocs/op
	// BenchmarkResolve-12    	    3452	    320228 ns/op	   57235 B/op	    3686 allocs/op
	// BenchmarkResolve-12    	    3250	    311136 ns/op	   57281 B/op	    3686 allocs/op
	// BenchmarkResolve-12    	    4326	    257411 ns/op	   50270 B/op	    2843 allocs/op
	// BenchmarkResolve-12    	    4885	    226071 ns/op	   46005 B/op	    2544 allocs/op
	// BenchmarkResolve-12    	    5059	    219532 ns/op	   42292 B/op	    2446 allocs/op
	// BenchmarkResolve-12    	    8761	    115397 ns/op	   17432 B/op	     306 allocs/op
	// BenchmarkResolve-12    	    8962	    112175 ns/op	   17424 B/op	     306 allocs/op
	// BenchmarkResolve-12    	    9361	    107861 ns/op	   17409 B/op	     306 allocs/op
	// BenchmarkResolve-12    	   12471	     88927 ns/op	    2857 B/op	     136 allocs/op

	// On desktop
	// BenchmarkResolve-16    	    2259	    503592 ns/op	   62823 B/op	    4340 allocs/op
	// BenchmarkResolve-16    	    2306	    454063 ns/op	   57633 B/op	    3686 allocs/op
	// BenchmarkResolve-16    	    3400	    305631 ns/op	   50040 B/op	    2817 allocs/op
	// BenchmarkResolve-16    	    3860	    303078 ns/op	   46153 B/op	    2544 allocs/op
	// BenchmarkResolve-16    	    4406	    265315 ns/op	   40399 B/op	    2326 allocs/op
	// BenchmarkResolve-16    	    8398	    150171 ns/op	   17443 B/op	     306 allocs/op
	// BenchmarkResolve-16    	    7682	    136384 ns/op	   17475 B/op	     306 allocs/op
	// BenchmarkResolve-16    	    9787	    118577 ns/op	    7559 B/op	     145 allocs/op
	// BenchmarkResolve-16    	    9458	    115545 ns/op	    2928 B/op	     136 allocs/op

	s, _ := ParseSchema(TestExecSchemaRequestWithFieldsData{}, M{}, nil)

	f, err := os.Create("memprofile")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

	for i := 0; i < b.N; i++ {
		_, errs := s.Resolve(schemaQuery, ResolveOptions{})
		for _, err := range errs {
			panic(err)
		}
	}

	// runtime.GC()
	// if err := pprof.WriteHeapProfile(f); err != nil {
	// 	log.Fatal("could not write memory profile: ", err)
	// }
}

func BenchmarkBytecodeResolve(b *testing.B) {
	s, _ := ParseSchema(TestExecSchemaRequestWithFieldsData{}, M{}, nil)
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

	for i := 0; i < b.N; i++ {
		_, errs := ctx.BytecodeResolve(query, opts)
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

func BenchmarkResolveTime(b *testing.B) {
	s, _ := ParseSchema(TestExecTimeIOData{}, M{}, nil)

	now := time.Now()
	testTimeInput := []byte{}
	timeToString(&testTimeInput, now)
	query := `{foo(t: "` + string(testTimeInput) + `")}`

	for i := 0; i < b.N; i++ {
		_, errs := s.Resolve(query, ResolveOptions{})
		for _, err := range errs {
			panic(err)
		}
	}
}

type HelloWorldSchema struct {
	Hello string
}

func BenchmarkBytecodeHelloWorldResolve(b *testing.B) {
	s, _ := ParseSchema(HelloWorldSchema{Hello: "World"}, M{}, nil)
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

	for i := 0; i < b.N; i++ {
		_, errs := ctx.BytecodeResolve(query, opts)
		for _, err := range errs {
			panic(err)
		}
	}
}

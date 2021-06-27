package graphql

import (
	"log"
	"os"
	"runtime/pprof"
	"testing"
)

func BenchmarkCheckNames(b *testing.B) {
	// On laptop
	// BenchmarkCheckNames-12    	    2277	    480462 ns/op	  189581 B/op	    2343 allocs/op
	// BenchmarkCheckNames-12    	   35962	     32972 ns/op	   17992 B/op	     339 allocs/op

	// On desktop
	// BenchmarkCheckNames-16    	   13941	     86455 ns/op	   23152 B/op	     993 allocs/op

	for i := 0; i < b.N; i++ {
		ParseQueryAndCheckNames(schemaQuery, nil)
	}
}

func BenchmarkResolve(b *testing.B) {
	// On laptop
	// BenchmarkResolve-12    	     854	   1383447 ns/op	  833377 B/op	   11670 allocs/op // First ran
	// BenchmarkResolve-12    	     852	   1379526 ns/op	  833150 B/op	   11668 allocs/op // Placed some resolver global variables in global scope
	// BenchmarkResolve-12    	     915	   1283598 ns/op	  782547 B/op	   10384 allocs/op // Use path from Ctx
	// BenchmarkResolve-12    	     886	   1308011 ns/op	  782452 B/op	   10379 allocs/op // Use array for value
	// BenchmarkResolve-12    	    1202	    998317 ns/op	  313687 B/op	    6064 allocs/op // Reduced a lot of string usage
	// BenchmarkResolve-12    	    1294	    898636 ns/op	  307930 B/op	    5686 allocs/op // Change value formatting to allocate less
	// BenchmarkResolve-12    	    3206	    345997 ns/op	   57292 B/op	    3686 allocs/op
	// BenchmarkResolve-12    	    3452	    320228 ns/op	   57235 B/op	    3686 allocs/op
	// BenchmarkResolve-12    	    3250	    311136 ns/op	   57281 B/op	    3686 allocs/op
	// BenchmarkResolve-12    	    4326	    257411 ns/op	   50270 B/op	    2843 allocs/op

	// On desktop
	// BenchmarkResolve-16    	    2259	    503592 ns/op	   62823 B/op	    4340 allocs/op
	// BenchmarkResolve-16    	    2306	    454063 ns/op	   57633 B/op	    3686 allocs/op

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
}

func BenchmarkEncodeString(b *testing.B) {
	// BenchmarkEncodeString-12    	 7684732	       151.1 ns/op	       0 B/op	       0 allocs/op
	// BenchmarkEncodeString-12    	 9391905	       121.1 ns/op	       0 B/op	       0 allocs/op

	inputString1 := []byte("abc")
	inputString2 := []byte("Some long string that includes spaces  and a .")
	inputString3 := []byte(`Wow this includes \\ and && and <> and ""`)
	out := []byte{}

	for i := 0; i < b.N; i++ {
		stringToJson(inputString1, &out)
		stringToJson(inputString2, &out)
		stringToJson(inputString3, &out)
		out = out[:0]
	}
}

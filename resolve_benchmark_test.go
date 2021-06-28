package graphql

import (
	"log"
	"os"
	"runtime/pprof"
	"testing"
	"time"
)

func BenchmarkQueryParser(b *testing.B) {
	// BenchmarkQueryParser-12    	   35697	     33170 ns/op	   17536 B/op	     331 allocs/op
	// BenchmarkQueryParser-12    	   37735	     30622 ns/op	   17488 B/op	     329 allocs/op
	// BenchmarkQueryParser-12    	   35721	     30887 ns/op	   10793 B/op	     273 allocs/op
	// BenchmarkQueryParser-12    	   51865	     19334 ns/op	   10770 B/op	     273 allocs/op

	f, err := os.Create("memprofile")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

	iter := &iterT{resErrors: []ErrorWLocation{}, selections: make([]selectionSet, 100)}
	for i := range iter.selections {
		iter.selections[i] = make(selectionSet, 5)
	}

	for i := 0; i < b.N; i++ {
		iter.parseQuery(schemaQuery)
	}
}

func BenchmarkResolve(b *testing.B) {
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

	// On desktop
	// BenchmarkResolve-16    	    2259	    503592 ns/op	   62823 B/op	    4340 allocs/op
	// BenchmarkResolve-16    	    2306	    454063 ns/op	   57633 B/op	    3686 allocs/op
	// BenchmarkResolve-16    	    3400	    305631 ns/op	   50040 B/op	    2817 allocs/op
	// BenchmarkResolve-16    	    3860	    303078 ns/op	   46153 B/op	    2544 allocs/op
	// BenchmarkResolve-16    	    4406	    265315 ns/op	   40399 B/op	    2326 allocs/op
	// BenchmarkResolve-16    	    8398	    150171 ns/op	   17443 B/op	     306 allocs/op

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
	inputString1 := []byte("abc")
	inputString2 := []byte("Some long string that includes spaces  and a .")
	inputString3 := []byte(`Wow this includes \\ and && and <> and ""`)
	inputString4 := []byte("The `String` scalar type represents textual data, represented as UTF-8 character sequences. The String type is most often used by GraphQL to represent free-form human-readable text.")
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

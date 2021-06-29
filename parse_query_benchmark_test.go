package graphql

import (
	"testing"
)

func BenchmarkParseName(b *testing.B) {
	// On laptop
	// BenchmarkParseName-12    	 5842974	       200.4 ns/op	      16 B/op	       2 allocs/op
	// BenchmarkParseName-12    	22152402	        47.90 ns/op	      16 B/op	       2 allocs/op

	buff := []byte{}

	validName := iterT{
		data: "_Banana ",
	}
	invalidValidName := iterT{
		data: "0Banana ",
	}

	for i := 0; i < b.N; i++ {
		v, _ := validName.parseName(buff[:0])
		if string(v) != "_Banana" {
			panic("parseName did not return expected value \"_Banana\"")
		}
		invalidValidName.parseName(buff[:0])

		validName.charNr = 0
		invalidValidName.charNr = 0
	}
}

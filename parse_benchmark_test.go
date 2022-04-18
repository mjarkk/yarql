package yarql

import (
	"testing"
)

func BenchmarkValidGraphQlName(b *testing.B) {
	// On laptop
	// BenchmarkValidGraphQlName-12    	 2822854	       438.1 ns/op	      48 B/op	       3 allocs/op
	// BenchmarkValidGraphQlName-12    	13395151	        85.27 ns/op	      48 B/op	       3 allocs/op

	validName := []byte("BananaHead")
	invalidName := []byte("_BananaHead")
	invalidName2 := []byte("0BananaHead")
	invalidName3 := []byte("Banana & Head")
	for i := 0; i < b.N; i++ {
		validGraphQlName(validName)
		validGraphQlName(invalidName)
		validGraphQlName(invalidName2)
		validGraphQlName(invalidName3)
	}
}

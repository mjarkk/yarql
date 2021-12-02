package helpers

import "testing"

func BenchmarkEncodeString(b *testing.B) {
	inputString1 := "abc"
	inputString2 := "Some long string that includes spaces  and a ."
	inputString3 := `Wow this includes \\ and && and <> and ""`
	inputString4 := "The `String` scalar type represents textual data, represented as UTF-8 character sequences. The String type is most often used by GraphQL to represent free-form human-readable text."
	out := []byte{}

	for i := 0; i < b.N; i++ {
		StringToJSON(inputString1, &out)
		StringToJSON(inputString2, &out)
		StringToJSON(inputString3, &out)
		StringToJSON(inputString4, &out)
	}
}

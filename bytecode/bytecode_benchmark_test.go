package bytecode

import (
	"hash/fnv"
	"testing"
)

func BenchmarkQueryParser(b *testing.B) {
	// BenchmarkQueryParser-12    	  267092	      4363 ns/op	       0 B/op	       0 allocs/op

	ctx := ParserCtx{
		Res:               make([]byte, 2048),
		FragmentLocations: make([]int, 8),
		Query:             []byte(schemaQuery),
		charNr:            0,
		Errors:            []error{},
		Hasher:            fnv.New32(),
	}

	for i := 0; i < b.N; i++ {
		ctx.ParseQueryToBytecode(nil)
		if len(ctx.Errors) > 0 {
			panic(ctx.Errors[len(ctx.Errors)-1])
		}
	}
}

var schemaQuery = `
query IntrospectionQuery {
	__schema {
		queryType {
			name
		}
		mutationType {
			name
		}
		subscriptionType {
			name
		}
		types {
			...FullType
		}
		directives {
			name
			description
			locations
			args {
				...InputValue
			}
		}
	}
}

fragment FullType on __Type {
	kind
	name
	description
	fields(includeDeprecated: true) {
		name
		description
		args {
			...InputValue
		}
		type {
			...TypeRef
		}
		isDeprecated
		deprecationReason
	}
	inputFields {
		...InputValue
	}
	interfaces {
		...TypeRef
	}
	enumValues(includeDeprecated: true) {
		name
		description
		isDeprecated
		deprecationReason
	}
	possibleTypes {
		...TypeRef
	}
}

fragment InputValue on __InputValue {
	name
	description
	type {
		...TypeRef
	}
	defaultValue
}

fragment TypeRef on __Type {
	kind
	name
	ofType {
		kind
		name
		ofType {
			kind
			name
			ofType {
				kind
				name
				ofType {
					kind
					name
					ofType {
						kind
						name
						ofType {
							kind
							name
							ofType {
								kind
								name
							}
						}
					}
				}
			}
		}
	}
}
`

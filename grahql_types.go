package graphql

//
// Types represent:
// https://spec.graphql.org/June2018/#sec-Schema-Introspection
//

var _ = TypeRename(QLSchema{}, "__Schema")

type QLSchema struct {
	Types            []QLType      `json:"types"`
	QueryType        *QLType       `json:"queryType"`
	MutationType     *QLType       `json:"mutationType"`
	SubscriptionType *QLType       `json:"subscriptionType"`
	Directives       []QLDirective `json:"directives"`
}

type IsDeprecatedArgs struct {
	IncludeDeprecated bool `json:"includeDeprecated"`
}

var _ = TypeRename(QLType{}, "__Type")

type QLType struct {
	// TODO make this a enum of type __TypeKind
	//
	// Options:
	// "SCALAR"
	// "OBJECT"
	// "INTERFACE"
	// "UNION"
	// "ENUM"
	// "INPUT_OBJECT"
	// "LIST"
	// "NON_NULL"
	Kind string `json:"kind"`

	Name        *string `json:"name"`
	Description *string `json:"description"`

	// OBJECT and INTERFACE only
	Fields func(IsDeprecatedArgs) []QLField `json:"-"`
	// For testing perposes mainly
	JSONFields []QLField `json:"fields" gqlignore:"true"`

	// OBJECT only
	Interfaces []QLType `json:"interfaces"`

	// INTERFACE and UNION only
	PossibleTypes []QLType `json:"possibleTypes"`

	// ENUM only
	EnumValues func(IsDeprecatedArgs) []QLEnumValue `json:"-"`

	// INPUT_OBJECT only
	InputFields []QLInputValue `json:"inputFields"`

	// NON_NULL and LIST only
	OfType *QLType `json:"ofType"`
}

var _ = TypeRename(QLField{}, "__Field")

type QLField struct {
	Name              string         `json:"name"`
	Description       *string        `json:"description"`
	Args              []QLInputValue `json:"args"`
	Type              QLType         `json:"type"`
	IsDeprecated      bool           `json:"isDeprecated"`
	DeprecationReason *string        `json:"deprecationReason"`
}

var _ = TypeRename(QLEnumValue{}, "__EnumValue")

type QLEnumValue struct {
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

var _ = TypeRename(QLInputValue{}, "__InputValue")

type QLInputValue struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Type         QLType  `json:"type"`
	DefaultValue *string `json:"defaultValue"`
}

var _ = TypeRename(QLDirective{}, "__Directive")

type QLDirective struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`

	// TODO make this a enum of type __DirectiveLocation
	//
	// Options:
	// "QUERY"
	// "MUTATION"
	// "SUBSCRIPTION"
	// "FIELD"
	// "FRAGMENT_DEFINITION"
	// "FRAGMENT_SPREAD"
	// "INLINE_FRAGMENT"
	// "SCHEMA"
	// "SCALAR"
	// "OBJECT"
	// "FIELD_DEFINITION"
	// "ARGUMENT_DEFINITION"
	// "INTERFACE"
	// "UNION"
	// "ENUM"
	// "ENUM_VALUE"
	// "INPUT_OBJECT"
	// "INPUT_FIELD_DEFINITION"
	Locations []string       `json:"locations"`
	Args      []QLInputValue `json:"args"`
}

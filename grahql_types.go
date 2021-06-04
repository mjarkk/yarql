package graphql

import h "github.com/mjarkk/go-graphql/helpers"

//
// Types represent:
// https://spec.graphql.org/June2018/#sec-Schema-Introspection
//

var _ = TypeRename(qlSchema{}, "__Schema")

type qlSchema struct {
	Types            []qlType      `json:"types"`
	QueryType        *qlType       `json:"queryType"`
	MutationType     *qlType       `json:"mutationType"`
	SubscriptionType *qlType       `json:"subscriptionType"`
	Directives       []qlDirective `json:"directives"`
}

type isDeprecatedArgs struct {
	IncludeDeprecated bool `json:"includeDeprecated"`
}

type __TypeKind uint8

const (
	typeKindScalar __TypeKind = iota
	typeKindObject
	typeKindInterface
	typeKindUnion
	typeKindEnum
	typeKindInputObject
	typeKindList
	typeKindNonNull
)

var _ = RegisterEnum(map[string]__TypeKind{
	"SCALAR":       typeKindScalar,
	"OBJECT":       typeKindObject,
	"INTERFACE":    typeKindInterface,
	"UNION":        typeKindUnion,
	"ENUM":         typeKindEnum,
	"INPUT_OBJECT": typeKindInputObject,
	"LIST":         typeKindList,
	"NON_NULL":     typeKindNonNull,
})

func (kind __TypeKind) String() string {
	switch kind {
	case typeKindScalar:
		return "SCALAR"
	case typeKindObject:
		return "OBJECT"
	case typeKindInterface:
		return "INTERFACE"
	case typeKindUnion:
		return "UNION"
	case typeKindEnum:
		return "ENUM"
	case typeKindInputObject:
		return "INPUT_OBJECT"
	case typeKindList:
		return "LIST"
	case typeKindNonNull:
		return "NON_NULL"
	}
	return ""
}

var _ = TypeRename(qlType{}, "__Type")

type qlType struct {
	Kind     __TypeKind `json:"-"`
	JSONKind string     `json:"kind" gqIgnore:"true"`

	Name        *string `json:"name"`
	Description *string `json:"description"`

	// OBJECT and INTERFACE only
	Fields func(isDeprecatedArgs) []qlField `json:"-"`
	// For testing perposes mainly
	JSONFields []qlField `json:"fields" gqIgnore:"true"`

	// OBJECT only
	Interfaces []qlType `json:"interfaces"`

	// INTERFACE and UNION only
	PossibleTypes []qlType `json:"possibleTypes"`

	// ENUM only
	EnumValues func(isDeprecatedArgs) []qlEnumValue `json:"-"`

	// INPUT_OBJECT only
	InputFields func() []qlInputValue `json:"-"`
	// For testing perposes mainly
	JSONInputFields []qlField `json:"inputFields" gqIgnore:"true"`

	// NON_NULL and LIST only
	OfType *qlType `json:"ofType"`
}

var _ = TypeRename(qlField{}, "__Field")

type qlField struct {
	Name              string         `json:"name"`
	Description       *string        `json:"description"`
	Args              []qlInputValue `json:"args"`
	Type              qlType         `json:"type"`
	IsDeprecated      bool           `json:"isDeprecated"`
	DeprecationReason *string        `json:"deprecationReason"`
}

var _ = TypeRename(qlEnumValue{}, "__EnumValue")

type qlEnumValue struct {
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

var _ = TypeRename(qlInputValue{}, "__InputValue")

type qlInputValue struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Type         qlType  `json:"type"`
	DefaultValue *string `json:"defaultValue"`
}

type __DirectiveLocation uint8

const (
	directiveLocationQuery __DirectiveLocation = iota
	directiveLocationMutation
	directiveLocationSubscription
	directiveLocationField
	directiveLocationFragmentDefinition
	directiveLocationFragmentSpread
	directiveLocationInlineFragment
	directiveLocationSchema
	directiveLocationScalar
	directiveLocationObject
	directiveLocationFieldDefinition
	directiveLocationArgumentDefinition
	directiveLocationInterface
	directiveLocationUnion
	directiveLocationEnum
	directiveLocationEnumValue
	directiveLocationInputObject
	directiveLocationInputFieldDefinition
)

var _ = RegisterEnum(map[string]__DirectiveLocation{
	"QUERY":                  directiveLocationQuery,
	"MUTATION":               directiveLocationMutation,
	"SUBSCRIPTION":           directiveLocationSubscription,
	"FIELD":                  directiveLocationField,
	"FRAGMENT_DEFINITION":    directiveLocationFragmentDefinition,
	"FRAGMENT_SPREAD":        directiveLocationFragmentSpread,
	"INLINE_FRAGMENT":        directiveLocationInlineFragment,
	"SCHEMA":                 directiveLocationSchema,
	"SCALAR":                 directiveLocationScalar,
	"OBJECT":                 directiveLocationObject,
	"FIELD_DEFINITION":       directiveLocationFieldDefinition,
	"ARGUMENT_DEFINITION":    directiveLocationArgumentDefinition,
	"INTERFACE":              directiveLocationInterface,
	"UNION":                  directiveLocationUnion,
	"ENUM":                   directiveLocationEnum,
	"ENUM_VALUE":             directiveLocationEnumValue,
	"INPUT_OBJECT":           directiveLocationInputObject,
	"INPUT_FIELD_DEFINITION": directiveLocationInputFieldDefinition,
})

var _ = TypeRename(qlDirective{}, "__Directive")

type qlDirective struct {
	Name          string                `json:"name"`
	Description   *string               `json:"description"`
	Locations     []__DirectiveLocation `json:"-"`
	JSONLocations []string              `json:"locations" gqIgnore:"true"`
	Args          []qlInputValue        `json:"args"`
}

var scalars = map[string]qlType{
	"Boolean": {Kind: typeKindScalar, Name: h.StrPtr("Boolean"), Description: h.StrPtr("The `Boolean` scalar type represents `true` or `false`.")},
	"Int":     {Kind: typeKindScalar, Name: h.StrPtr("Int"), Description: h.StrPtr("The Int scalar type represents a signed 32‐bit numeric non‐fractional value.")},
	"Float":   {Kind: typeKindScalar, Name: h.StrPtr("Float"), Description: h.StrPtr("The Float scalar type represents signed double‐precision fractional values as specified by IEEE 754.")},
	"String":  {Kind: typeKindScalar, Name: h.StrPtr("String"), Description: h.StrPtr("The `String` scalar type represents textual data, represented as UTF-8 character sequences. The String type is most often used by GraphQL to represent free-form human-readable text.")},
	// "ID": {Kind: TypeKindScalar, Name: h.StrPtr("ID"), Description: h.StrPtr("The ID scalar type represents a unique identifier, often used to refetch an object or as the key for a cache")},
}

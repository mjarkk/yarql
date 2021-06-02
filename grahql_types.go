package graphql

import h "github.com/mjarkk/go-graphql/helpers"

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

type __TypeKind uint8

const (
	TypeKindScalar __TypeKind = iota
	TypeKindObject
	TypeKindInterface
	TypeKindUnion
	TypeKindEnum
	TypeKindInputObject
	TypeKindList
	TypeKindNonNull
)

var _ = RegisterEnum(map[string]__TypeKind{
	"SCALAR":       TypeKindScalar,
	"OBJECT":       TypeKindObject,
	"INTERFACE":    TypeKindInterface,
	"UNION":        TypeKindUnion,
	"ENUM":         TypeKindEnum,
	"INPUT_OBJECT": TypeKindInputObject,
	"LIST":         TypeKindList,
	"NON_NULL":     TypeKindNonNull,
})

func (kind __TypeKind) String() string {
	switch kind {
	case TypeKindScalar:
		return "SCALAR"
	case TypeKindObject:
		return "OBJECT"
	case TypeKindInterface:
		return "INTERFACE"
	case TypeKindUnion:
		return "UNION"
	case TypeKindEnum:
		return "ENUM"
	case TypeKindInputObject:
		return "INPUT_OBJECT"
	case TypeKindList:
		return "LIST"
	case TypeKindNonNull:
		return "NON_NULL"
	}
	return ""
}

var _ = TypeRename(QLType{}, "__Type")

type QLType struct {
	Kind     __TypeKind `json:"-"`
	JSONKind string     `json:"kind" gqlignore:"true"`

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
	InputFields func() []QLInputValue `json:"-"`
	// For testing perposes mainly
	JSONInputFields []QLField `json:"inputFields" gqlignore:"true"`

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

type __DirectiveLocation uint8

const (
	DirectiveLocationQuery __DirectiveLocation = iota
	DirectiveLocationMutation
	DirectiveLocationSubscription
	DirectiveLocationField
	DirectiveLocationFragmentDefinition
	DirectiveLocationFragmentSpread
	DirectiveLocationInlineFragment
	DirectiveLocationSchema
	DirectiveLocationScalar
	DirectiveLocationObject
	DirectiveLocationFieldDefinition
	DirectiveLocationArgumentDefinition
	DirectiveLocationInterface
	DirectiveLocationUnion
	DirectiveLocationEnum
	DirectiveLocationEnumValue
	DirectiveLocationInputObject
	DirectiveLocationInputFieldDefinition
)

var _ = RegisterEnum(map[string]__DirectiveLocation{
	"QUERY":                  DirectiveLocationQuery,
	"MUTATION":               DirectiveLocationMutation,
	"SUBSCRIPTION":           DirectiveLocationSubscription,
	"FIELD":                  DirectiveLocationField,
	"FRAGMENT_DEFINITION":    DirectiveLocationFragmentDefinition,
	"FRAGMENT_SPREAD":        DirectiveLocationFragmentSpread,
	"INLINE_FRAGMENT":        DirectiveLocationInlineFragment,
	"SCHEMA":                 DirectiveLocationSchema,
	"SCALAR":                 DirectiveLocationScalar,
	"OBJECT":                 DirectiveLocationObject,
	"FIELD_DEFINITION":       DirectiveLocationFieldDefinition,
	"ARGUMENT_DEFINITION":    DirectiveLocationArgumentDefinition,
	"INTERFACE":              DirectiveLocationInterface,
	"UNION":                  DirectiveLocationUnion,
	"ENUM":                   DirectiveLocationEnum,
	"ENUM_VALUE":             DirectiveLocationEnumValue,
	"INPUT_OBJECT":           DirectiveLocationInputObject,
	"INPUT_FIELD_DEFINITION": DirectiveLocationInputFieldDefinition,
})

var _ = TypeRename(QLDirective{}, "__Directive")

type QLDirective struct {
	Name          string                `json:"name"`
	Description   *string               `json:"description"`
	Locations     []__DirectiveLocation `json:"-"`
	JSONLocations []string              `json:"locations" gqlignore:"true"`
	Args          []QLInputValue        `json:"args"`
}

var scalars = map[string]QLType{
	"Boolean": QLType{Kind: TypeKindScalar, Name: h.StrPtr("Boolean"), Description: h.StrPtr("The `Boolean` scalar type represents `true` or `false`.")},
	"Int":     QLType{Kind: TypeKindScalar, Name: h.StrPtr("Int"), Description: h.StrPtr("The Int scalar type represents a signed 32‐bit numeric non‐fractional value.")},
	"Float":   QLType{Kind: TypeKindScalar, Name: h.StrPtr("Float"), Description: h.StrPtr("The Float scalar type represents signed double‐precision fractional values as specified by IEEE 754.")},
	"String":  QLType{Kind: TypeKindScalar, Name: h.StrPtr("String"), Description: h.StrPtr("The `String` scalar type represents textual data, represented as UTF-8 character sequences. The String type is most often used by GraphQL to represent free-form human-readable text.")},
	// "ID": QLType{Kind: TypeKindScalar, Name: h.StrPtr("ID"), Description: h.StrPtr("The ID scalar type represents a unique identifier, often used to refetch an object or as the key for a cache")},
}

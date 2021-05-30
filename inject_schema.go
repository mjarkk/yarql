package graphql

import (
	"log"
	"reflect"
	"sort"

	h "github.com/mjarkk/go-graphql/helpers"
)

func (s *Schema) injectQLTypes(ctx *parseCtx) {
	ref, err := ctx.check(reflect.TypeOf(QLSchema{}))
	if err != nil {
		log.Fatal(err)
	}

	contents := reflect.ValueOf(s.GetQLSchema())
	ref.customObjValue = &contents

	s.rootQuery.objContents["__schema"] = ref

	// TODO add:
	// __type(name: String!): __Type
	// __typename
}

func (s *Schema) GetQLSchema() QLSchema {
	res := QLSchema{
		Types:      s.GetAllQLTypes(),
		Directives: []QLDirective{},
		QueryType: &QLType{
			Kind:        "OBJECT",
			Name:        h.StrPtr(s.rootQuery.typeName),
			Description: h.StrPtr(""),
			Fields: func(args IsDeprecatedArgs) []QLField {
				res := []QLField{}
				for key, item := range s.rootQuery.objContents {
					res = append(res, QLField{
						Name: key,
						Args: []QLInputValue{},
						Type: *wrapQLTypeInNonNull(s.objToQLType(item)),
					})
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
				return res
			},
			Interfaces:    []QLType{},
			PossibleTypes: nil,
			EnumValues:    nil,
			InputFields:   nil,
			OfType:        nil,
		},
		MutationType: &QLType{
			Kind:        "OBJECT",
			Name:        h.StrPtr(s.rootMethod.typeName),
			Description: h.StrPtr(""),
		},
	}

	res.SubscriptionType = nil // TODO: We currently don't support subscriptions

	return res
}

func (s *Schema) GetAllQLTypes() []QLType {
	res := []QLType{}

	for _, type_ := range s.types {
		obj, _ := s.objToQLType(type_)
		res = append(res, *obj)
	}
	for _, in := range s.inTypes {
		obj, _ := s.inputToQLType(in)
		res = append(res, *obj)
	}
	sort.Slice(res, func(a int, b int) bool { return *res[a].Name < *res[b].Name })

	return append(res,
		ScalarBoolean,
		ScalarInt,
		ScalarFloat,
		ScalarString,
		// ScalarID,
	)
}

func wrapQLTypeInNonNull(type_ *QLType, isNonNull bool) *QLType {
	if !isNonNull {
		return type_
	}
	return &QLType{
		Kind:   "NON_NULL",
		OfType: type_,
	}
}

var (
	ScalarBoolean = QLType{Kind: "SCALAR", Name: h.StrPtr("Boolean"), Description: h.StrPtr("The `Boolean` scalar type represents `true` or `false`.")}
	ScalarInt     = QLType{Kind: "SCALAR", Name: h.StrPtr("Int"), Description: h.StrPtr("The Int scalar type represents a signed 32‐bit numeric non‐fractional value.")}
	ScalarFloat   = QLType{Kind: "SCALAR", Name: h.StrPtr("Float"), Description: h.StrPtr("The Float scalar type represents signed double‐precision fractional values as specified by IEEE 754.")}
	ScalarString  = QLType{Kind: "SCALAR", Name: h.StrPtr("String"), Description: h.StrPtr("The `String` scalar type represents textual data, represented as UTF-8 character sequences. The String type is most often used by GraphQL to represent free-form human-readable text.")}
	// ScalarID      = QLType{Kind: "SCALAR", Name: h.StrPtr("ID"), Description: h.StrPtr("The ID scalar type represents a unique identifier, often used to refetch an object or as the key for a cache")}
)

func (s *Schema) inputToQLType(in *Input) (res *QLType, isNonNull bool) {
	switch in.kind {
	case reflect.Struct:
		isNonNull = true
		res = &QLType{
			Kind:        "INPUT_OBJECT",
			Name:        h.StrPtr(in.structName),
			Description: h.StrPtr(""),
			InputFields: []QLInputValue{},
		}
	default:
		// TODO: Support more types
	}

	return
}

func (s *Schema) objToQLType(item *Obj) (res *QLType, isNonNull bool) {
	switch item.valueType {
	case valueTypeUndefined:
		// WUT??, we'll just look away and continue as if nothing happend
		// FIXME: maybe we should return an error here
	case valueTypeArray:
		res = &QLType{
			Kind:        "LIST",
			Name:        nil,
			Description: nil,
			OfType:      wrapQLTypeInNonNull(s.objToQLType(item.innerContent)),
		}
	case valueTypeObjRef:
		return s.objToQLType(s.types[item.typeName])
	case valueTypeObj:
		isNonNull = true
		res = &QLType{
			Kind:        "OBJECT",
			Name:        h.StrPtr(item.typeName),
			Description: h.StrPtr(""),
			Fields: func(args IsDeprecatedArgs) []QLField {
				res := []QLField{}
				for key, item := range item.objContents {
					res = append(res, QLField{
						Name: key,
						Args: []QLInputValue{},
						Type: *wrapQLTypeInNonNull(s.objToQLType(item)),
					})
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
				return res
			},
		}
	case valueTypeData:
		isNonNull = true
		switch item.dataValueType {
		case reflect.Bool:
			res = &ScalarBoolean
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
			res = &ScalarInt
		case reflect.Float32, reflect.Float64:
			res = &ScalarFloat
		case reflect.String:
			res = &ScalarString
		default:
			res = &QLType{Kind: "SCALAR", Name: h.StrPtr(""), Description: h.StrPtr("")}
		}
	case valueTypePtr:
		res, _ := s.objToQLType(item.innerContent)
		return res, false
	case valueTypeMethod:
		// TODO
		res, isNonNull = s.objToQLType(&item.method.outType)
		if !item.method.isTypeMethod {
			isNonNull = false
		}
	}

	return
}

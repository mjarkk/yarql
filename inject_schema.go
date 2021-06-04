package graphql

import (
	"log"
	"reflect"
	"sort"

	h "github.com/mjarkk/go-graphql/helpers"
)

func (s *Schema) injectQLTypes(ctx *parseCtx) {
	// Inject __Schema
	ref, err := ctx.check(reflect.TypeOf(QLSchema{}))
	if err != nil {
		log.Fatal(err)
	}

	contents := reflect.ValueOf(s.getQLSchema())
	ref.customObjValue = &contents
	s.rootQuery.objContents["__schema"] = ref

	// Inject __type(name: String!): __Type
	typeResolver := func(ctx *Ctx, args struct{ Name string }) *QLType {
		return ctx.schema.getTypeByName(args.Name, true, true)
	}
	typeResolverReflection := reflect.ValueOf(typeResolver)
	functionObj, err := ctx.checkStructFieldFunc("__type", typeResolverReflection.Type())
	if err != nil {
		log.Fatal(err)
	}

	functionObj.customObjValue = &typeResolverReflection
	s.rootQuery.objContents["__type"] = functionObj
}

func (s *Schema) getQLSchema() QLSchema {
	res := QLSchema{
		Types:      s.getAllQLTypes(),
		Directives: []QLDirective{},
		QueryType: &QLType{
			Kind:        TypeKindObject,
			Name:        h.StrPtr(s.rootQuery.typeName),
			Description: h.StrPtr(""),
			Fields: func(args IsDeprecatedArgs) []QLField {
				res := []QLField{}
				for key, item := range s.rootQuery.objContents {
					res = append(res, QLField{
						Name: key,
						Args: s.getObjectArgs(item),
						Type: *wrapQLTypeInNonNull(s.objToQLType(item)),
					})
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
				return res
			},
			Interfaces: []QLType{},
		},
		MutationType: &QLType{
			Kind:        TypeKindObject,
			Name:        h.StrPtr(s.rootMethod.typeName),
			Description: h.StrPtr(""),
		},
	}

	res.SubscriptionType = nil // TODO: We currently don't support subscriptions

	return res
}

func (s *Schema) getAllQLTypes() []QLType {
	res := []QLType{}

	for _, type_ := range s.types {
		obj, _ := s.objToQLType(type_)
		res = append(res, *obj)
	}
	for _, in := range s.inTypes {
		obj, _ := s.inputToQLType(in)
		res = append(res, *obj)
	}
	for _, enum := range definedEnums {
		res = append(res, enumToQlType(enum))
	}
	for _, scalar := range scalars {
		res = append(res, scalar)
	}
	sort.Slice(res, func(a int, b int) bool { return *res[a].Name < *res[b].Name })

	return res
}

func (s *Schema) getTypeByName(name string, includeInputTypes, includeOutputTypes bool) *QLType {
	// FIXME: Make one gigantic map on schema creation with all the types below
	scalars, ok := scalars[name]
	if ok {
		return &scalars
	}

	enum, ok := definedEnums[name]
	if ok {
		res := enumToQlType(enum)
		return &res
	}

	if includeOutputTypes {
		type_, ok := s.types[name]
		if ok {
			res, _ := s.objToQLType(type_)
			return res
		}
	}
	if includeInputTypes {
		inType, ok := s.inTypes[name]
		if ok {
			obj, _ := s.inputToQLType(inType)
			return obj
		}
	}
	return nil
}

func wrapQLTypeInNonNull(type_ *QLType, isNonNull bool) *QLType {
	if !isNonNull {
		return type_
	}
	return &QLType{
		Kind:   TypeKindNonNull,
		OfType: type_,
	}
}

func (s *Schema) inputToQLType(in *Input) (res *QLType, isNonNull bool) {
	switch in.kind {
	case reflect.Struct:
		isNonNull = true

		res = &QLType{
			Kind:        TypeKindInputObject,
			Name:        h.StrPtr(in.structName),
			Description: h.StrPtr(""),
			InputFields: func() []QLInputValue {
				res := []QLInputValue{}
				for key, item := range in.structContent {
					res = append(res, QLInputValue{
						Name:         key,
						Description:  h.StrPtr(""),
						Type:         *wrapQLTypeInNonNull(s.inputToQLType(&item)),
						DefaultValue: nil, // We do not support this atm
					})
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
				return res
			},
		}
	case reflect.Array, reflect.Slice:
		res = &QLType{
			Kind:   TypeKindList,
			OfType: wrapQLTypeInNonNull(s.inputToQLType(in.elem)),
		}
	case reflect.Ptr:
		// This basically sets the isNonNull to false
		res, _ = s.inputToQLType(in.elem)
	case reflect.Bool:
		isNonNull = true
		rawRes := scalars["Boolean"]
		res = &rawRes
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		isNonNull = true
		rawRes := scalars["Int"]
		res = &rawRes
	case reflect.Float32, reflect.Float64:
		isNonNull = true
		rawRes := scalars["Float"]
		res = &rawRes
	case reflect.String:
		isNonNull = true
		rawRes := scalars["String"]
		res = &rawRes
	default:
		isNonNull = true
		res = &QLType{Kind: TypeKindScalar, Name: h.StrPtr(""), Description: h.StrPtr("")}
	}
	return
}

func (s *Schema) getObjectArgs(item *Obj) []QLInputValue {
	res := []QLInputValue{}
	if item.valueType == valueTypeMethod {
		for key, value := range item.method.inFields {
			res = append(res, QLInputValue{
				Name:         key,
				Description:  h.StrPtr(""),
				Type:         *wrapQLTypeInNonNull(s.inputToQLType(&value.input)),
				DefaultValue: nil,
			})
		}
		sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
	}
	return res
}

func (s *Schema) objToQLType(item *Obj) (res *QLType, isNonNull bool) {
	switch item.valueType {
	case valueTypeUndefined:
		// WUT??, we'll just look away and continue as if nothing happend
		// FIXME: maybe we should return an error here
	case valueTypeArray:
		res = &QLType{
			Kind:   TypeKindList,
			OfType: wrapQLTypeInNonNull(s.objToQLType(item.innerContent)),
		}
	case valueTypeObjRef:
		return s.objToQLType(s.types[item.typeName])
	case valueTypeObj:
		isNonNull = true
		res = &QLType{
			Kind:        TypeKindObject,
			Name:        h.StrPtr(item.typeName),
			Description: h.StrPtr(""),
			Fields: func(args IsDeprecatedArgs) []QLField {
				res := []QLField{}
				for key, innerItem := range item.objContents {
					res = append(res, QLField{
						Name: key,
						Args: s.getObjectArgs(innerItem),
						Type: *wrapQLTypeInNonNull(s.objToQLType(innerItem)),
					})
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
				return res
			},
		}
	case valueTypeData:
		isNonNull = true
		var rawRes QLType
		switch item.dataValueType {
		case reflect.Bool:
			rawRes = scalars["Boolean"]
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
			rawRes = scalars["Int"]
		case reflect.Float32, reflect.Float64:
			rawRes = scalars["Float"]
		case reflect.String:
			rawRes = scalars["String"]
		default:
			rawRes = QLType{Kind: TypeKindScalar, Name: h.StrPtr(""), Description: h.StrPtr("")}
		}
		res = &rawRes
	case valueTypeEnum:
		isNonNull = true
		enumType := enumToQlType(definedEnums[item.enumTypeName])
		res = &enumType
	case valueTypePtr:
		// This basically sets the isNonNull to false
		res, _ := s.objToQLType(item.innerContent)
		return res, false
	case valueTypeMethod:
		res, isNonNull = s.objToQLType(&item.method.outType)
		if !item.method.isTypeMethod {
			isNonNull = false
		}
	}

	return
}

func enumToQlType(enum enum) QLType {
	name := enum.contentType.Name()
	return QLType{
		Kind: TypeKindEnum,
		Name: &name,
		EnumValues: func(args IsDeprecatedArgs) []QLEnumValue {
			res := []QLEnumValue{}
			for key := range enum.keyValue {
				res = append(res, QLEnumValue{
					Name:              key,
					Description:       h.StrPtr(""),
					IsDeprecated:      false,
					DeprecationReason: nil,
				})
			}
			sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
			return res
		},
	}
}

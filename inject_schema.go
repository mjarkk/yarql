package graphql

import (
	"log"
	"reflect"
	"sort"

	h "github.com/mjarkk/go-graphql/helpers"
)

func (s *Schema) injectQLTypes(ctx *parseCtx) {
	// Inject __Schema
	ref, err := ctx.check(reflect.TypeOf(qlSchema{}), false)
	if err != nil {
		log.Fatal(err)
	}

	contents := reflect.ValueOf(s.getQLSchema())
	ref.customObjValue = &contents
	ref.qlFieldName = []byte("__schema")

	s.rootQuery.objContents[getObjKey(ref.qlFieldName)] = ref

	// Inject __type(name: String!): __Type
	typeResolver := func(ctx *Ctx, args struct{ Name string }) *qlType {
		if ctx.bytecodeCtx != nil {
			return ctx.bytecodeCtx.schema.getTypeByName(args.Name)
		}
		return ctx.schema.getTypeByName(args.Name)
	}
	typeResolverReflection := reflect.ValueOf(typeResolver)
	functionObj, err := ctx.checkStructFieldFunc("__type", typeResolverReflection.Type(), false, -1)
	if err != nil {
		log.Fatal(err)
	}

	functionObj.customObjValue = &typeResolverReflection
	functionObj.qlFieldName = []byte("__type")
	s.rootQuery.objContents[getObjKey(functionObj.qlFieldName)] = functionObj
}

func (s *Schema) getQLSchema() qlSchema {
	res := qlSchema{
		Types:      s.getAllQLTypes,
		Directives: s.getDirectives(),
		QueryType: &qlType{
			Kind:        typeKindObject,
			Name:        h.StrPtr(s.rootQuery.typeName),
			Description: h.PtrToEmptyStr,
			Fields: func(isDeprecatedArgs) []qlField {
				fields, ok := s.graphqlObjFields[s.rootQuery.typeName]
				if ok {
					return fields
				}

				res := []qlField{}
				for _, item := range s.rootQuery.objContents {
					res = append(res, qlField{
						Name: string(item.qlFieldName),
						Args: s.getObjectArgs(item),
						Type: *wrapQLTypeInNonNull(s.objToQLType(item)),
					})
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })

				s.graphqlObjFields[s.rootQuery.typeName] = res
				return res
			},
			Interfaces: []qlType{},
		},
		MutationType: &qlType{
			Kind:        typeKindObject,
			Name:        h.StrPtr(s.rootMethod.typeName),
			Description: h.PtrToEmptyStr,
		},
	}

	// TODO: We currently don't support subscriptions
	res.SubscriptionType = nil

	return res
}

func (s *Schema) getDirectives() []qlDirective {
	return []qlDirective{
		{
			Name:        "skip",
			Description: h.StrPtr("Directs the executor to skip this field or fragment when the `if` argument is true."),
			Locations: []__DirectiveLocation{
				directiveLocationField,
				directiveLocationFragmentSpread,
				directiveLocationInlineFragment,
			},
			Args: []qlInputValue{
				{
					Name:        "if",
					Description: h.StrPtr("Skipped when true."),
					Type:        ScalarBoolean,
				},
			},
		},
		{
			Name:        "include",
			Description: h.StrPtr("Directs the executor to include this field or fragment only when the `if` argument is true."),
			Locations: []__DirectiveLocation{
				directiveLocationField,
				directiveLocationFragmentSpread,
				directiveLocationInlineFragment,
			},
			Args: []qlInputValue{
				{
					Name:        "if",
					Description: h.StrPtr("Included when true."),
					Type:        ScalarBoolean,
				},
			},
		},
	}
}

func (s *Schema) getAllQLTypes() []qlType {
	if s.graphqlTypesList == nil {
		s.graphqlTypesList = make([]qlType, len(s.types)+len(s.inTypes)+len(definedEnums)+len(scalars))
		idx := 0

		for _, type_ := range s.types {
			obj, _ := s.objToQLType(type_)
			s.graphqlTypesList[idx] = *obj
			idx++
		}
		for _, in := range s.inTypes {
			obj, _ := s.inputToQLType(in)
			s.graphqlTypesList[idx] = *obj
			idx++
		}
		for _, enum := range definedEnums {
			s.graphqlTypesList[idx] = enum.qlType
			idx++
		}
		for _, scalar := range scalars {
			s.graphqlTypesList[idx] = scalar
			idx++
		}
		sort.Slice(s.graphqlTypesList, func(a int, b int) bool { return *s.graphqlTypesList[a].Name < *s.graphqlTypesList[b].Name })
	}

	return s.graphqlTypesList
}

func (s *Schema) getTypeByName(name string) *qlType {
	if s.graphqlTypesMap == nil {
		// Build up s.graphqlTypesMap
		s.graphqlTypesMap = map[string]qlType{}
		all := s.getAllQLTypes()
		for _, type_ := range all {
			s.graphqlTypesMap[*type_.Name] = type_
		}
	}

	type_, ok := s.graphqlTypesMap[name]
	if ok {
		return &type_
	}
	return nil
}

func wrapQLTypeInNonNull(type_ *qlType, isNonNull bool) *qlType {
	if !isNonNull {
		return type_
	}
	return &qlType{
		Kind:   typeKindNonNull,
		OfType: type_,
	}
}

func (s *Schema) inputToQLType(in *input) (res *qlType, isNonNull bool) {
	if in.isID {
		res = &ScalarID
		return
	} else if in.isTime {
		res = &ScalarTime
		isNonNull = true
		return
	} else if in.isFile {
		res = &ScalarFile
		return
	}

	switch in.kind {
	case reflect.Struct:
		isNonNull = true

		res = &qlType{
			Kind:        typeKindInputObject,
			Name:        h.StrPtr(in.structName),
			Description: h.PtrToEmptyStr,
			InputFields: func() []qlInputValue {
				res := make([]qlInputValue, len(in.structContent))
				i := 0
				for key, item := range in.structContent {
					res[i] = qlInputValue{
						Name:         key,
						Description:  h.PtrToEmptyStr,
						Type:         *wrapQLTypeInNonNull(s.inputToQLType(&item)),
						DefaultValue: nil, // We do not support this atm
					}
					i++
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
				return res
			},
		}
	case reflect.Array, reflect.Slice:
		res = &qlType{
			Kind:   typeKindList,
			OfType: wrapQLTypeInNonNull(s.inputToQLType(in.elem)),
		}
	case reflect.Ptr:
		// Basically sets the isNonNull to false
		res, _ = s.inputToQLType(in.elem)
	case reflect.Bool:
		isNonNull = true
		res = &ScalarBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		isNonNull = true
		if in.isID {
			res = &ScalarID
		} else {
			res = &ScalarInt
		}
	case reflect.Float32, reflect.Float64:
		isNonNull = true
		res = &ScalarFloat
	case reflect.String:
		isNonNull = true
		res = &ScalarString
	default:
		isNonNull = true
		res = &qlType{
			Kind:        typeKindScalar,
			Name:        h.PtrToEmptyStr,
			Description: h.PtrToEmptyStr,
		}
	}
	return
}

func (s *Schema) getObjectArgs(item *obj) []qlInputValue {
	res := []qlInputValue{}
	if item.valueType == valueTypeMethod {
		for key, value := range item.method.inFields {
			res = append(res, qlInputValue{
				Name:         key,
				Description:  h.PtrToEmptyStr,
				Type:         *wrapQLTypeInNonNull(s.inputToQLType(&value.input)),
				DefaultValue: nil,
			})
		}
		sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })
	}
	return res
}

func (s *Schema) objToQLType(item *obj) (res *qlType, isNonNull bool) {
	switch item.valueType {
	case valueTypeUndefined:
		// WUT??, we'll just look away and continue as if nothing happend
		// FIXME: maybe we should return an error here
		return
	case valueTypeArray:
		res = &qlType{
			Kind:   typeKindList,
			OfType: wrapQLTypeInNonNull(s.objToQLType(item.innerContent)),
		}
		return
	case valueTypeObjRef:
		return s.objToQLType(s.types[item.typeName])
	case valueTypeObj:
		isNonNull = true
		res = &qlType{
			Kind:        typeKindObject,
			Name:        &item.typeName,
			Description: h.PtrToEmptyStr,
			Fields: func(args isDeprecatedArgs) []qlField {
				fields, ok := s.graphqlObjFields[item.typeName]
				if ok {
					return fields
				}

				res := []qlField{}
				for _, innerItem := range item.objContents {
					res = append(res, qlField{
						Name: string(innerItem.qlFieldName),
						Args: s.getObjectArgs(innerItem),
						Type: *wrapQLTypeInNonNull(s.objToQLType(innerItem)),
					})
				}
				sort.Slice(res, func(a int, b int) bool { return res[a].Name < res[b].Name })

				s.graphqlObjFields[item.typeName] = res
				return res
			},
			Interfaces: []qlType{},
		}
		return
	case valueTypeEnum:
		enumType := definedEnums[item.enumTypeIndex].qlType
		res = &enumType
		return res, true
	case valueTypePtr:
		// This basically sets the isNonNull to false
		res, _ := s.objToQLType(item.innerContent)
		return res, false
	case valueTypeMethod:
		res, isNonNull = s.objToQLType(&item.method.outType)
		if !item.method.isTypeMethod {
			isNonNull = false
		}
		return
	default:
		return resolveObjToScalar(item), true
	}
}

func resolveObjToScalar(item *obj) *qlType {
	var res qlType
	switch item.valueType {
	case valueTypeData:
		if item.isID {
			res = ScalarID
		} else {
			switch item.dataValueType {
			case reflect.Bool:
				res = ScalarBoolean
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
				res = ScalarInt
			case reflect.Float32, reflect.Float64:
				res = ScalarFloat
			case reflect.String:
				res = ScalarString
			default:
				res = qlType{Kind: typeKindScalar, Name: h.PtrToEmptyStr, Description: h.PtrToEmptyStr}
			}
		}
		return &res
	case valueTypeTime:
		res = ScalarTime
		return &res
	}
	return nil
}

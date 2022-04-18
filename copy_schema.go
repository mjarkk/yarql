package yarql

import (
	"reflect"

	"github.com/mjarkk/yarql/bytecode"
	"github.com/mjarkk/yarql/helpers"
	"github.com/valyala/fastjson"
)

// Copy is meant to be used to create a pool of schema objects
// The function itself is quiet slow so don't use this function in every request
func (s *Schema) Copy() *Schema {
	if !s.parsed {
		panic("Schema has not been parsed yet, call Parse before attempting to copy it")
	}

	types := s.types.copy()
	interfaces := s.interfaces.copy()

	enums := make([]enum, len(s.definedEnums))
	for idx, enum := range s.definedEnums {
		enums[idx] = *enum.copy()
	}

	directives := map[DirectiveLocation][]*Directive{}
	for key, value := range s.definedDirectives {
		directivesToAdd := make([]*Directive, len(value))
		for idx, directive := range value {
			directivesToAdd[idx] = directive.copy()
		}
		directives[key] = directivesToAdd
	}

	res := &Schema{
		parsed: true,

		types:      *types,
		inTypes:    *s.inTypes.copy(),
		interfaces: *interfaces,

		rootQuery:         s.rootQuery.copy(),
		rootQueryValue:    s.rootQueryValue,
		rootMethod:        s.rootMethod.copy(),
		rootMethodValue:   s.rootMethodValue,
		MaxDepth:          s.MaxDepth,
		definedEnums:      enums,
		definedDirectives: directives,

		Result:           make([]byte, len(s.Result)),
		graphqlTypesMap:  nil,
		graphqlTypesList: nil,
		graphqlObjFields: map[string][]qlField{},
	}

	res.ctx = s.ctx.copy(res)

	return res
}

func (ctx *Ctx) copy(schema *Schema) *Ctx {
	res := &Ctx{
		schema:                   schema,
		query:                    *bytecode.NewParserCtx(),
		charNr:                   ctx.charNr,
		context:                  nil,
		path:                     []byte{},
		getFormFile:              ctx.getFormFile,
		operatorHasArguments:     ctx.operatorHasArguments,
		operatorArgumentsStartAt: ctx.operatorArgumentsStartAt,
		tracingEnabled:           ctx.tracingEnabled,
		tracing:                  ctx.tracing,
		prefRecordingStartTime:   ctx.prefRecordingStartTime,
		rawVariables:             ctx.rawVariables,
		variablesParsed:          false,
		variablesJSONParser:      &fastjson.Parser{},
		variables:                nil,
		reflectValues:            [256]reflect.Value{},
		currentReflectValueIdx:   0,
		funcInputs:               []reflect.Value{},
		values:                   nil,
	}
	res.ctxReflection = reflect.ValueOf(res)
	return res
}

func (m *Directive) copy() *Directive {
	var parsedMethod *objMethod
	if m.parsedMethod != nil {
		parsedMethod = m.parsedMethod.copy()
	}
	return &Directive{
		Name:             m.Name,
		Where:            m.Where,
		Method:           m.Method,           // Maybe TODO
		methodReflection: m.methodReflection, // Maybe TODO
		parsedMethod:     parsedMethod,
		Description:      m.Description,
	}
}

func (m *enum) copy() *enum {
	res := &enum{
		contentType: m.contentType,
		contentKind: m.contentKind,
		typeName:    m.typeName,
		entries:     []enumEntry{},
		qlType:      *m.qlType.copy(),
	}
	for _, entry := range m.entries {
		res.entries = append(res.entries, enumEntry{
			keyBytes: entry.keyBytes[:],
			key:      entry.key,
			value:    entry.value, // Maybe TODO
		})
	}

	return res
}

func (m *qlType) copy() *qlType {
	res := &qlType{
		Kind:          m.Kind,
		Fields:        m.Fields,
		PossibleTypes: m.PossibleTypes,
		EnumValues:    m.EnumValues,
		InputFields:   m.InputFields,

		// The json fields are not relevant in the context this method is used
	}
	if m.Name != nil {
		res.Name = helpers.StrPtr("")
		*res.Name = *m.Name
	}
	if m.Description != nil {
		res.Name = helpers.StrPtr("")
		*res.Name = *m.Name
	}
	if m.Interfaces != nil {
		res.Interfaces = make([]qlType, len(m.Interfaces))
		for idx, interf := range m.Interfaces {
			res.Interfaces[idx] = *interf.copy()
		}
	}
	if m.OfType != nil {
		res.OfType = m.OfType.copy()
	}
	return res
}

func (m *inputMap) copy() *inputMap {
	res := inputMap{}
	for key, value := range *m {
		res[key] = value.copy()
	}
	return &res
}

func (t *types) copy() *types {
	res := types{}

	for k, v := range *t {
		res[k] = v.copy()
	}

	return &res
}

func (o *obj) copy() *obj {
	res := obj{
		valueType:      o.valueType,
		typeName:       o.typeName,
		typeNameBytes:  o.typeNameBytes[:],
		goTypeName:     o.goTypeName,
		goPkgPath:      o.goPkgPath,
		qlFieldName:    o.qlFieldName[:],
		customObjValue: o.customObjValue, // maybe TODO
		structFieldIdx: o.structFieldIdx,
		dataValueType:  o.dataValueType,
		isID:           o.isID,
		enumTypeIndex:  o.enumTypeIndex,
	}

	if o.innerContent != nil {
		res.innerContent = o.innerContent.copy()
	}

	if o.method != nil {
		res.method = o.method.copy()
	}

	if o.objContents != nil {
		res.objContents = map[uint32]*obj{}
		for key, value := range o.objContents {
			res.objContents[key] = value.copy()
		}
	}

	if o.implementations != nil {
		for _, impl := range o.implementations {
			res.implementations = append(res.implementations, impl.copy())
		}
	}

	return &res
}

func (m *objMethod) copy() *objMethod {
	res := objMethod{
		isTypeMethod:   m.isTypeMethod,
		goFunctionName: m.goFunctionName,
		goType:         m.goType,
		checkedIns:     m.checkedIns,
		outNr:          m.outNr,
		outType:        *m.outType.copy(),
	}
	if m.errorOutNr != nil {
		errOutNr := 0
		res.errorOutNr = &errOutNr

		*res.errorOutNr = *m.errorOutNr
	}

	if m.ins != nil {
		res.ins = make([]baseInput, len(m.ins))
		for i, in := range m.ins {
			res.ins[i] = *in.copy()
		}
	}

	if m.inFields != nil {
		res.inFields = map[string]referToInput{}
		for key, value := range m.inFields {
			res.inFields[key] = *value.copy()
		}
	}

	return &res
}

func (m *referToInput) copy() *referToInput {
	return &referToInput{
		inputIdx: m.inputIdx,
		input:    *m.input.copy(),
	}
}

func (m *input) copy() *input {
	var structContent map[string]input

	if m.structContent != nil {
		structContent = map[string]input{}
		for key, value := range m.structContent {
			structContent[key] = *value.copy()
		}
	}

	var elem *input
	if m.elem != nil {
		elem = m.elem.copy()
	}

	return &input{
		kind:             m.kind,
		isEnum:           m.isEnum,
		enumTypeIndex:    m.enumTypeIndex,
		isID:             m.isID,
		isFile:           m.isFile,
		isTime:           m.isTime,
		goFieldIdx:       m.goFieldIdx,
		gqFieldName:      m.gqFieldName,
		elem:             elem,
		isStructPointers: m.isStructPointers,
		structName:       m.structName,
		structContent:    structContent,
	}
}

func (m *baseInput) copy() *baseInput {
	res := &baseInput{
		isCtx: m.isCtx,
	}
	if m.goType != nil {
		reflectType := reflect.TypeOf(0)
		res.goType = &reflectType

		*res.goType = *m.goType
	}

	return res
}

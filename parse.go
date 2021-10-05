package graphql

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type types map[string]*obj

func (t *types) Add(obj obj) obj {
	if obj.valueType != valueTypeObj {
		panic("Can only add struct types to list")
	}

	val := *t
	val[obj.typeName] = &obj
	*t = val

	return obj.getRef()
}

func (t *types) Get(key string) (*obj, bool) {
	val, ok := (*t)[key]
	return val, ok
}

// Schema defines the graphql schema
type Schema struct {
	parsed bool

	types             types
	inTypes           inputMap
	rootQuery         *obj
	rootQueryValue    reflect.Value
	rootMethod        *obj
	rootMethodValue   reflect.Value
	MaxDepth          uint8 // Default 255
	definedEnums      []enum
	definedDirectives map[DirectiveLocation][]*Directive
	ctx               *Ctx

	// Zero alloc variables
	Result           []byte
	graphqlTypesMap  map[string]qlType
	graphqlTypesList []qlType
	graphqlObjFields map[string][]qlField
}

type valueType int

const (
	valueTypeUndefined valueType = iota
	valueTypeArray
	valueTypeObjRef
	valueTypeObj
	valueTypeData
	valueTypePtr
	valueTypeMethod
	valueTypeEnum
	valueTypeTime
)

type obj struct {
	valueType     valueType
	typeName      string
	typeNameBytes []byte
	pkgPath       string
	qlFieldName   []byte

	// Value type == valueTypeObj
	objContents    map[uint32]*obj
	customObjValue *reflect.Value // Mainly Graphql internal values like __schema

	// Value is inside struct
	structFieldIdx int

	// Value type == valueTypeArray || type == valueTypePtr
	innerContent *obj

	// Value type == valueTypeData
	dataValueType reflect.Kind
	isID          bool

	// Value type == valueTypeMethod
	method *objMethod

	// Value type == valueTypeEnum
	enumTypeIndex int
}

func getObjKey(key []byte) uint32 {
	hasher := fnv.New32()
	hasher.Write(key)
	return hasher.Sum32()
}

func (o *obj) getRef() obj {
	if o.valueType != valueTypeObj {
		panic("getRef can only be used on objects")
	}

	return obj{
		valueType:     valueTypeObjRef,
		typeName:      o.typeName,
		typeNameBytes: []byte(o.typeName),
	}
}

type objMethod struct {
	// Is this a function field inside this object or a method attached to the struct
	// true = func (*someStruct) ResolveFooBar() string {}
	// false = ResolveFooBar func() string
	isTypeMethod   bool
	goFunctionName string
	type_          reflect.Type

	ins        []baseInput             // The real function inputs
	inFields   map[string]referToInput // Contains all the fields of all the ins
	checkedIns bool                    // are the ins checked yet

	outNr      int
	outType    obj
	errorOutNr *int
}

type inputMap map[string]*input

type referToInput struct {
	inputIdx int // the method's argument index
	input    input
}

type input struct {
	kind reflect.Kind

	// Is this a custom type?
	isEnum        bool
	enumTypeIndex int
	isID          bool
	isFile        bool
	isTime        bool

	goFieldIdx  int
	gqFieldName string

	// kind == Slice, Array or Ptr
	elem *input

	// kind == struct
	isStructPointers bool
	structName       string
	structContent    map[string]input
}

type baseInput struct {
	isCtx bool
	type_ *reflect.Type
}

type SchemaOptions struct {
	noMethodEqualToQueryChecks bool // only used for for testing
	SkipGraphqlTypesInjection  bool
}

type parseCtx struct {
	schema             *Schema
	unknownTypesCount  int
	unknownInputsCount int
	parsedMethods      []*objMethod
}

func NewSchema() *Schema {
	s := &Schema{
		types:             types{},
		inTypes:           inputMap{},
		MaxDepth:          255,
		graphqlObjFields:  map[string][]qlField{},
		definedEnums:      []enum{},
		definedDirectives: map[DirectiveLocation][]*Directive{},
		Result:            make([]byte, 16384),
	}

	added, err := s.RegisterEnum(directiveLocationMap)
	if err != nil {
		panic("INTERNAL ERROR: " + err.Error())
	}
	if !added {
		panic("INTERNAL ERROR: directive locations ENUM should be added")
	}

	s.RegisterEnum(typeKindEnumMap)
	if err != nil {
		panic("INTERNAL ERROR: " + err.Error())
	}
	if !added {
		panic("INTERNAL ERROR: type kind ENUM should be added")
	}

	err = s.RegisterDirective(Directive{
		Name: "skip",
		Where: []DirectiveLocation{
			DirectiveLocationField,
			DirectiveLocationFragment,
			DirectiveLocationFragmentInline,
		},
		Method: func(args struct{ If bool }) DirectiveModifier {
			return DirectiveModifier{
				Skip: args.If,
			}
		},
		Description: "Directs the executor to skip this field or fragment when the `if` argument is true.",
	})
	if err != nil {
		panic("INTERNAL ERROR: " + err.Error())
	}

	err = s.RegisterDirective(Directive{
		Name: "include",
		Where: []DirectiveLocation{
			DirectiveLocationField,
			DirectiveLocationFragment,
			DirectiveLocationFragmentInline,
		},
		Method: func(args struct{ If bool }) DirectiveModifier {
			return DirectiveModifier{
				Skip: !args.If,
			}
		},
		Description: "Directs the executor to include this field or fragment only when the `if` argument is true.",
	})
	if err != nil {
		panic("INTERNAL ERROR: " + err.Error())
	}

	return s
}

func (s *Schema) Parse(queries interface{}, methods interface{}, options *SchemaOptions) error {
	s.rootQueryValue = reflect.ValueOf(queries)
	s.rootMethodValue = reflect.ValueOf(methods)

	ctx := &parseCtx{
		schema:        s,
		parsedMethods: []*objMethod{},
	}

	obj, err := ctx.check(reflect.TypeOf(queries), false)
	if err != nil {
		return err
	}
	if obj.valueType != valueTypeObjRef {
		return errors.New("input queries must be a struct")
	}
	s.rootQuery = s.types[obj.typeName]

	obj, err = ctx.check(reflect.TypeOf(methods), false)
	if err != nil {
		return err
	}
	if obj.valueType != valueTypeObjRef {
		return errors.New("input methods must be a struct")
	}
	s.rootMethod = s.types[obj.typeName]

	if options == nil || !options.noMethodEqualToQueryChecks {
		queryPkg := s.rootQuery.pkgPath + s.rootQuery.typeName
		methodPkg := s.rootMethod.pkgPath + s.rootMethod.typeName
		if queryPkg == methodPkg {
			return errors.New("method and query cannot be the same struct")
		}
	}

	if options == nil || !options.SkipGraphqlTypesInjection {
		s.injectQLTypes(ctx)
	}

	for _, method := range ctx.parsedMethods {
		err = ctx.checkFunctionIns(method)
		if err != nil {
			return err
		}
	}

	for _, directiveLocation := range ctx.schema.definedDirectives {
		for _, directive := range directiveLocation {
			if directive.parsedMethod.checkedIns {
				continue
			}

			err = ctx.checkFunctionIns(directive.parsedMethod)
			if err != nil {
				return err
			}
		}
	}

	s.ctx = newCtx(s)
	s.parsed = true

	return nil
}

func (c *parseCtx) check(t reflect.Type, hasIDTag bool) (*obj, error) {
	res := obj{
		typeNameBytes: []byte(t.Name()),
		typeName:      t.Name(),
		pkgPath:       t.PkgPath(),
	}

	if res.pkgPath == "time" && res.typeName == "Time" {
		res.valueType = valueTypeTime
		return &res, nil
	}

	switch t.Kind() {
	case reflect.Struct:
		res.valueType = valueTypeObj

		if res.typeName != "" {
			newName, ok := renamedTypes[res.typeName]
			if ok {
				res.typeName = newName
				res.typeNameBytes = []byte(newName)
			}

			v, ok := c.schema.types.Get(res.typeName)
			if ok {
				if v.pkgPath != res.pkgPath {
					return nil, fmt.Errorf("cannot have 2 structs with same type in structure: %s(%s) != %s(%s)", v.pkgPath, res.typeName, res.pkgPath, res.typeName)
				}

				res = v.getRef()
				return &res, nil
			}
		} else {
			c.unknownTypesCount++
			res.typeName = "__UnknownType" + strconv.Itoa(c.unknownTypesCount)
			res.typeNameBytes = []byte(res.typeName)
		}

		res.objContents = map[uint32]*obj{}

		typesInner := c.schema.types
		typesInner[res.typeName] = &res
		c.schema.types = typesInner

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			customName, obj, err := c.checkStructField(field, i)
			if err != nil {
				return nil, err
			}
			if obj != nil {
				name := formatGoNameToQL(field.Name)
				if customName != nil {
					name = *customName
				}
				obj.qlFieldName = []byte(name)

				res.objContents[getObjKey(obj.qlFieldName)] = obj
			}
		}
	case reflect.Array, reflect.Slice, reflect.Ptr:
		isPtr := t.Kind() == reflect.Ptr
		if isPtr {
			res.valueType = valueTypePtr
		} else {
			res.valueType = valueTypeArray
		}

		obj, err := c.check(t.Elem(), hasIDTag && isPtr)
		if err != nil {
			return nil, err
		}
		res.innerContent = obj
	case reflect.Func, reflect.Map, reflect.Chan, reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Interface, reflect.UnsafePointer:
		return nil, fmt.Errorf("unsupported value type %s", t.Kind().String())
	default:
		enumIndex, enum := c.schema.getEnum(t)
		if enum != nil {
			res.valueType = valueTypeEnum
			res.enumTypeIndex = enumIndex
		} else {
			res.valueType = valueTypeData
			res.dataValueType = t.Kind()

			if hasIDTag {
				res.isID = hasIDTag
				err := checkValidIDKind(res.dataValueType)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if res.valueType == valueTypeObj {
		for i := 0; i < t.NumMethod(); i++ {
			method := t.Method(i)
			methodObj, name, err := c.checkFunction(method.Name, method.Type, true, false)
			if err != nil {
				return nil, err
			} else if methodObj == nil {
				continue
			}

			qlFieldName := []byte(name)
			res.objContents[getObjKey(qlFieldName)] = &obj{
				qlFieldName:    qlFieldName,
				valueType:      valueTypeMethod,
				pkgPath:        method.PkgPath,
				structFieldIdx: i,
				method:         methodObj,
			}
		}

		res = c.schema.types.Add(res)
		// res is now a objptr pointing to the obj
	}

	return &res, nil
}

func (c *parseCtx) checkStructField(field reflect.StructField, idx int) (customName *string, obj *obj, err error) {
	if field.Anonymous {
		return nil, nil, nil
	}

	var ignore, isID bool
	customName, ignore, isID, err = parseFieldTagGQ(&field)
	if ignore || err != nil {
		return nil, nil, err
	}

	if field.Type.Kind() == reflect.Func {
		obj, err = c.checkStructFieldFunc(field.Name, field.Type, isID, idx)
	} else {
		obj, err = c.check(field.Type, isID)
	}

	if obj != nil {
		obj.structFieldIdx = idx
	}
	return
}

func (c *parseCtx) checkStructFieldFunc(fieldName string, type_ reflect.Type, hasIDTag bool, idx int) (*obj, error) {
	methodObj, _, err := c.checkFunction(fieldName, type_, false, hasIDTag)
	if err != nil {
		return nil, err
	} else if methodObj == nil {
		return nil, nil
	}
	return &obj{
		valueType:      valueTypeMethod,
		method:         methodObj,
		structFieldIdx: idx,
	}, nil
}

var ctxType = reflect.TypeOf(Ctx{})

func isCtx(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && ctxType.Name() == t.Name() && ctxType.PkgPath() == t.PkgPath()
}

func (c *parseCtx) checkFunctionInputStruct(field *reflect.StructField, idx int) (res input, skipThisField bool, err error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("%s, struct field: %s", err.Error(), field.Name)
	}

	if field.Anonymous {
		// skip field
		return res, true, nil
	}

	newName, ignore, isID, err := parseFieldTagGQ(field)
	if ignore {
		// skip field
		return res, true, nil
	}
	if err != nil {
		return res, false, wrapErr(err)
	}

	qlFieldName := formatGoNameToQL(field.Name)
	if newName != nil {
		qlFieldName = *newName
	}

	res, err = c.checkFunctionInput(field.Type, isID)
	if err != nil {
		return input{}, false, wrapErr(err)
	}

	res.goFieldIdx = idx
	res.gqFieldName = qlFieldName

	return
}

func (c *parseCtx) checkFunctionInput(t reflect.Type, hasIDTag bool) (input, error) {
	kind := t.Kind()
	res := input{
		kind: kind,
	}

	switch kind {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		enumIndex, enum := c.schema.getEnum(t)
		if enum != nil {
			res.isEnum = true
			res.enumTypeIndex = enumIndex
		} else if hasIDTag {
			res.isID = true
			err := checkValidIDKind(res.kind)
			if err != nil {
				return res, err
			}
		}
	case reflect.Ptr:
		if t.AssignableTo(reflect.TypeOf(&multipart.FileHeader{})) {
			// This is a file header, these are handled completely diffrent from a normal pointer
			res.isFile = true
			return res, nil
		}

		input, err := c.checkFunctionInput(t.Elem(), hasIDTag)
		if err != nil {
			return res, err
		}
		res.elem = &input
	case reflect.Array, reflect.Slice:
		input, err := c.checkFunctionInput(t.Elem(), false)
		if err != nil {
			return res, err
		}
		res.elem = &input
	case reflect.Struct:
		if t.AssignableTo(reflect.TypeOf(time.Time{})) {
			// This is a time property, these are handled completely diffrent from a normal struct
			return input{
				kind:   reflect.String,
				isTime: true,
			}, nil
		}

		structName := t.Name()
		if len(structName) == 0 {
			c.unknownInputsCount++
			structName = "__UnknownInput" + strconv.Itoa(c.unknownInputsCount)
		} else {
			newStructName, ok := renamedTypes[structName]
			if ok {
				structName = newStructName
			}
			_, equalTypeExist := c.schema.types[structName]
			if equalTypeExist {
				// types and inputs with the same name are not allowed in graphql, add __input as suffix
				// TODO allow this value to be filledin by the user
				structName = structName + "__input"
			}
		}

		_, ok := c.schema.inTypes[structName]
		if !ok {
			// Make sure the input types entry is set before looping over it's fields to fix the n+1 problem
			c.schema.inTypes[structName] = &res

			res.structName = structName
			res.structContent = map[string]input{}
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				input, skip, err := c.checkFunctionInputStruct(&field, i)
				if skip {
					continue
				}
				if err != nil {
					return res, err
				}
				res.structContent[input.gqFieldName] = input
			}
		}

		return input{
			kind:             kind,
			structName:       structName,
			isStructPointers: true,
		}, nil
	case reflect.Map, reflect.Func:
		// TODO: maybe we can do something with these
		fallthrough
	default:
		return res, fmt.Errorf("unsupported type %s", kind.String())
	}

	return res, nil
}

func (c *parseCtx) checkFunction(name string, t reflect.Type, isTypeMethod bool, hasIDTag bool) (*objMethod, string, error) {
	trimmedName := name

	if strings.HasPrefix(name, "Resolve") {
		if len(name) > 0 {
			trimmedName = strings.TrimPrefix(name, "Resolve")
			if isTypeMethod && strings.ToUpper(string(trimmedName[0]))[0] != trimmedName[0] {
				// Resolve name must start with a uppercase letter
				return nil, "", nil
			}
		} else if isTypeMethod {
			return nil, "", nil
		}
	} else if isTypeMethod {
		return nil, "", nil
	}

	if t.IsVariadic() {
		return nil, "", errors.New("function method cannot end with spread operator")
	}

	numOuts := t.NumOut()
	if numOuts == 0 {
		return nil, "", fmt.Errorf("%s no value returned", name)
	} else if numOuts > 2 {
		return nil, "", fmt.Errorf("%s cannot return more than 2 response values", name)
	}

	var outNr *int
	var outTypeObj *obj
	var hasErrorOut *int

	errInterface := reflect.TypeOf((*error)(nil)).Elem()
	for i := 0; i < numOuts; i++ {
		outType := t.Out(i)
		outKind := outType.Kind()
		if outKind == reflect.Interface {
			if outType.Implements(errInterface) {
				if hasErrorOut != nil {
					return nil, "", fmt.Errorf("%s cannot return multiple error types", name)
				} else {
					hasErrorOut = func(i int) *int {
						return &i
					}(i)
				}
			} else {
				return nil, "", fmt.Errorf("%s cannot return interface type", name)
			}
		} else {
			if outNr != nil {
				return nil, "", fmt.Errorf("%s cannot return multiple types of data", name)
			}

			outObj, err := c.check(outType, hasIDTag)
			if err != nil {
				return nil, "", err
			}

			outNr = func(i int) *int {
				return &i
			}(i)
			outTypeObj = outObj
		}
	}

	if outTypeObj == nil {
		return nil, "", fmt.Errorf("%s does not return usable data", name)
	}

	res := &objMethod{
		type_:          t,
		goFunctionName: name,
		isTypeMethod:   isTypeMethod,
		ins:            []baseInput{},
		inFields:       map[string]referToInput{},
		outNr:          *outNr,
		outType:        *outTypeObj,
		errorOutNr:     hasErrorOut,
	}
	c.parsedMethods = append(c.parsedMethods, res)
	return res, formatGoNameToQL(trimmedName), nil
}

func (c *parseCtx) checkFunctionIns(method *objMethod) error {
	totalInputs := method.type_.NumIn()
	for i := 0; i < totalInputs; i++ {
		iInList := i
		if method.isTypeMethod {
			if i == 0 {
				// First argument can be skipped if type method
				continue
			}
			iInList = i - 1
		}

		type_ := method.type_.In(i)
		input := baseInput{}
		typeKind := type_.Kind()
		if typeKind == reflect.Ptr && isCtx(type_.Elem()) {
			input.isCtx = true
		} else if isCtx(type_) {
			return fmt.Errorf("%s ctx argument must be a pointer", method.goFunctionName)
		} else if typeKind == reflect.Struct {
			input.type_ = &type_
			for i := 0; i < type_.NumField(); i++ {
				field := type_.Field(i)
				input, skip, err := c.checkFunctionInputStruct(&field, i)
				if skip {
					continue
				}
				if err != nil {
					return fmt.Errorf("%s, type %s (#%d)", err.Error(), type_.Name(), i)
				}

				method.inFields[input.gqFieldName] = referToInput{
					inputIdx: iInList,
					input:    input,
				}
			}
		} else {
			return fmt.Errorf("invalid struct item type %s (#%d)", type_.Name(), i)
		}

		method.ins = append(method.ins, input)
	}

	method.checkedIns = true
	return nil
}

func formatGoNameToQL(input string) string {
	if len(input) <= 1 {
		return strings.ToLower(input)
	}

	if input[1] == bytes.ToUpper([]byte{input[1]})[0] {
		// Don't change names like: INPUT to iNPUT
		return input
	}

	return string(bytes.ToLower([]byte{input[0]})) + input[1:]
}

func parseFieldTagGQ(field *reflect.StructField) (newName *string, ignore bool, isID bool, err error) {
	val, ok := field.Tag.Lookup("gq")
	if !ok {
		return
	}
	if val == "" {
		return
	}

	args := strings.Split(val, ",")
	nameArg := strings.TrimSpace(args[0])
	if nameArg != "" {
		if nameArg == "-" {
			ignore = true
			return
		}
		err = validGraphQlName([]byte(nameArg))
		newName = &nameArg
	}

	for _, modifier := range args[1:] {
		switch strings.ToLower(strings.TrimSpace(modifier)) {
		case "id":
			isID = true
		default:
			err = fmt.Errorf("unknown field tag gq argument: %s", modifier)
			return
		}
	}

	return
}

func validGraphQlName(name []byte) error {
	if len(name) == 0 {
		return errors.New("invalid graphql name")
	}
	for i, char := range name {
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= 'a' && char <= 'z' {
			continue
		}
		if i > 0 {
			if char >= '0' && char <= '9' {
				continue
			}
			if char == '_' {
				continue
			}
		}
		return errors.New("invalid graphql name")
	}
	return nil
}

func checkValidIDKind(kind reflect.Kind) error {
	switch kind {
	case reflect.String:
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return errors.New("strings and numbers can only be labeld with the ID property")
	}
	return nil
}

func (s *Schema) objToQlTypeName(item *obj, target *bytes.Buffer) {
	suffix := []byte{}

	qlType := wrapQLTypeInNonNull(s.objToQLType(item))

	for {
		switch qlType.Kind {
		case typeKindList:
			target.WriteByte('[')
			suffix = append(suffix, ']')
		case typeKindNonNull:
			suffix = append(suffix, '!')
		default:
			if qlType.Name != nil {
				target.WriteString(*qlType.Name)
			} else {
				target.Write([]byte("Unknown"))
			}
			if len(suffix) > 0 {
				target.Write(suffix)
			}
			return
		}
		qlType = qlType.OfType
	}
}

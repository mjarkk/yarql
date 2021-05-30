package graphql

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type Types map[string]*Obj

func (t *Types) Add(obj Obj) Obj {
	if obj.valueType != valueTypeObj {
		panic("Can only add struct types to list")
	}

	val := *t
	val[obj.typeName] = &obj
	*t = val

	return obj.getRef()
}

func (t *Types) Get(key string) (*Obj, bool) {
	val, ok := (*t)[key]
	return val, ok
}

// Schema defines the graphql schema
type Schema struct {
	types           Types
	inTypes         InputMap
	rootQuery       *Obj
	rootQueryValue  reflect.Value
	rootMethod      *Obj
	rootMethodValue reflect.Value
	m               sync.Mutex
	MaxDepth        uint8 // Default 255
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
)

type Obj struct {
	valueType valueType
	typeName  string
	pkgPath   string

	// Value type == valueTypeObj
	objContents    map[string]*Obj
	customObjValue *reflect.Value // Mainly Graphql internal values like __schema

	// Value is inside struct
	structFieldName string

	// Value type == valueTypeArray || type == valueTypePtr
	innerContent *Obj

	// Value type == valueTypeData
	dataValueType reflect.Kind

	// Value type == valueTypeMethod
	method *ObjMethod
}

func (o *Obj) getRef() Obj {
	if o.valueType != valueTypeObj {
		panic("getRef can only be used on objects")
	}

	return Obj{
		valueType: valueTypeObjRef,
		typeName:  o.typeName,
	}
}

type ObjMethod struct {
	// Is this a function field inside this object or a method attached to the struct
	// true = func (*someStruct) ResolveFooBar() string {}
	// false = ResolveFooBar func() string
	isTypeMethod bool

	ins      []BaseInput             // The real function inputs
	inFields map[string]referToInput // Contains all the fields of all the ins

	outNr      int
	outType    Obj
	errorOutNr *int
}

type InputMap map[string]*Input

type referToInput struct {
	inputIdx int
	input    Input
}

type Input struct {
	kind reflect.Kind

	goFieldName string // NOTE: In case of a re-named type this name will be diffrent
	gqFieldName string

	// kind == Slice, Array or Ptr
	elem *Input

	// kind == struct
	isStructPointers bool
	structName       string
	structContent    map[string]Input
}

type BaseInput struct {
	isCtx bool
	type_ *reflect.Type
}

type SchemaOptions struct {
	noMethodEqualToQueryChecks bool // internal for testing
	SkipGraphqlTypesInjection  bool
}

type parseCtx struct {
	// TODO: Maybe change this to objects as the types defined here are always objects
	types              *Types
	inTypes            *InputMap
	unknownTypesCount  int
	unkonwnInputsCount int
}

func newParseCtx() *parseCtx {
	return &parseCtx{
		types:   &Types{},
		inTypes: &InputMap{},
	}
}

func ParseSchema(queries interface{}, methods interface{}, options *SchemaOptions) (*Schema, error) {
	res := Schema{
		types:           Types{},
		inTypes:         InputMap{},
		rootQueryValue:  reflect.ValueOf(queries),
		rootMethodValue: reflect.ValueOf(methods),
		MaxDepth:        255,
	}

	ctx := parseCtx{
		types:   &res.types,
		inTypes: &res.inTypes,
	}

	obj, err := ctx.check(reflect.TypeOf(queries))
	if err != nil {
		return nil, err
	}
	if obj.valueType != valueTypeObjRef {
		return nil, errors.New("Input queries must be a struct")
	}
	res.rootQuery = res.types[obj.typeName]

	obj, err = ctx.check(reflect.TypeOf(methods))
	if err != nil {
		return nil, err
	}
	if obj.valueType != valueTypeObjRef {
		return nil, errors.New("Input methods must be a struct")
	}
	res.rootMethod = res.types[obj.typeName]

	if options == nil || !options.noMethodEqualToQueryChecks {
		queryPkg := res.rootQuery.pkgPath + res.rootQuery.typeName
		methodPkg := res.rootMethod.pkgPath + res.rootMethod.typeName
		if queryPkg == methodPkg {
			return nil, errors.New("method and query cannot be the same struct")
		}
	}

	if options == nil || !options.SkipGraphqlTypesInjection {
		res.injectQLTypes(&ctx)
	}

	return &res, nil
}

func (c *parseCtx) check(t reflect.Type) (*Obj, error) {
	res := Obj{
		typeName: t.Name(),
		pkgPath:  t.PkgPath(),
	}

	switch t.Kind() {
	case reflect.Struct:
		res.valueType = valueTypeObj

		if res.typeName != "" {
			newName, ok := renamedTypes[res.typeName]
			if ok {
				res.typeName = newName
			}

			v, ok := c.types.Get(res.typeName)
			if ok {
				if v.pkgPath != res.pkgPath {
					return nil, fmt.Errorf("cannot have 2 structs with same type in structure: %s(%s) != %s(%s)", v.pkgPath, res.typeName, res.pkgPath, res.typeName)
				}

				res = v.getRef()
				return &res, nil
			}
		} else {
			c.unknownTypesCount++
			res.typeName = fmt.Sprintf("__UnknownType%d", c.unknownTypesCount)
		}

		res.objContents = map[string]*Obj{}

		typesInner := *c.types
		typesInner[res.typeName] = &res
		*c.types = typesInner

		for i := 0; i < t.NumField(); i++ {
			err := func(field reflect.StructField) error {
				if field.Anonymous {
					return nil
				}

				val, ok := field.Tag.Lookup("gqIgnore")
				if ok && valueLooksTrue(val) {
					return nil
				}

				newName, hasNameRewrite := field.Tag.Lookup("gqName")
				name := formatGoNameToQL(field.Name)
				if hasNameRewrite {
					name = newName
				}

				if field.Type.Kind() == reflect.Func {
					methodObj, _, err := c.checkFunction(field.Name, field.Type, false)
					if err != nil {
						return err
					} else if methodObj == nil {
						return nil
					}

					res.objContents[name] = &Obj{
						valueType:       valueTypeMethod,
						pkgPath:         field.PkgPath,
						structFieldName: field.Name,
						method:          methodObj,
					}
				} else {
					obj, err := c.check(field.Type)
					if err != nil {
						return err
					}
					obj.structFieldName = field.Name
					res.objContents[name] = obj
				}
				return nil
			}(t.Field(i))
			if err != nil {
				return nil, err
			}
		}
	case reflect.Array, reflect.Slice, reflect.Ptr:
		if t.Kind() == reflect.Ptr {
			res.valueType = valueTypePtr
		} else {
			res.valueType = valueTypeArray
		}

		obj, err := c.check(t.Elem())
		if err != nil {
			return nil, err
		}
		res.innerContent = obj
	case reflect.Func, reflect.Map, reflect.Chan, reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Interface, reflect.UnsafePointer:
		return nil, fmt.Errorf("unsupported value type %s", t.Kind().String())
	default:
		res.valueType = valueTypeData
		res.dataValueType = t.Kind()
	}

	if res.valueType == valueTypeObj {
		for i := 0; i < t.NumMethod(); i++ {
			method := t.Method(i)
			methodName := method.Name
			methodObj, name, err := c.checkFunction(methodName, method.Type, true)
			if err != nil {
				return nil, err
			} else if methodObj == nil {
				continue
			}

			res.objContents[name] = &Obj{
				valueType:       valueTypeMethod,
				pkgPath:         method.PkgPath,
				structFieldName: methodName,
				method:          methodObj,
			}
		}
	}

	if res.valueType == valueTypeObj {
		res = c.types.Add(res)
		// res is now a objptr pointing to the obj
	}

	return &res, nil
}

var simpleCtx = reflect.TypeOf(Ctx{})

func isCtx(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && simpleCtx.Name() == t.Name() && simpleCtx.PkgPath() == t.PkgPath()
}

func (c *parseCtx) checkFunctionInputStruct(field *reflect.StructField) (res Input, skipThisField bool, err error) {
	if field.Anonymous {
		// skip field
		return res, true, nil
	}

	val, ok := field.Tag.Lookup("gqIgnore")
	if ok && valueLooksTrue(val) {
		// skip field
		return res, true, nil
	}

	qlFieldName := formatGoNameToQL(field.Name)
	newQlFieldName, hasNameRewrite := field.Tag.Lookup("gqName")
	if hasNameRewrite {
		qlFieldName = newQlFieldName
	}

	res, err = c.checkFunctionInput(field.Type)
	if err != nil {
		return Input{}, false, fmt.Errorf("%s, struct field: %s", err.Error(), field.Name)
	}

	res.goFieldName = field.Name
	res.gqFieldName = qlFieldName

	return
}

func (c *parseCtx) checkFunctionInput(t reflect.Type) (Input, error) {
	kind := t.Kind()
	res := Input{
		kind: kind,
	}

	switch kind {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		// We don't have todo anything here just go to the next input
	case reflect.Ptr, reflect.Array, reflect.Slice:
		input, err := c.checkFunctionInput(t.Elem())
		if err != nil {
			return res, err
		}
		res.elem = &input
	case reflect.Struct:
		structName := t.Name()
		if len(structName) == 0 {
			c.unkonwnInputsCount++
			structName = fmt.Sprintf("__UnknownInput%d", c.unkonwnInputsCount)
		} else {
			newStructName, ok := renamedTypes[structName]
			if ok {
				structName = newStructName
			}
		}

		types := *c.inTypes
		_, ok := types[structName]
		if !ok {
			// Make sure the input types entry is set before looping over it's fields to fix the n+1 problem
			types[structName] = &res
			*c.inTypes = types

			res.structName = structName
			res.structContent = map[string]Input{}
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				input, skip, err := c.checkFunctionInputStruct(&field)
				if skip {
					continue
				}
				if err != nil {
					return res, err
				}
				res.structContent[input.gqFieldName] = input
			}
		}

		return Input{
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

func (c *parseCtx) checkFunction(name string, t reflect.Type, isTypeMethod bool) (*ObjMethod, string, error) {
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
		return nil, "", errors.New("function method cannot end with spread argument")
	}

	ins := []BaseInput{}
	inFields := map[string]referToInput{}

	totalInputs := t.NumIn()
	for i := 0; i < totalInputs; i++ {
		iInList := i
		if isTypeMethod {
			if i == 0 {
				// First argument can be skipped if type method
				continue
			}
			iInList = i - 1
		}

		type_ := t.In(i)
		input := BaseInput{}
		typeKind := type_.Kind()
		if typeKind == reflect.Ptr && isCtx(type_.Elem()) {
			input.isCtx = true
		} else if isCtx(type_) {
			return nil, "", fmt.Errorf("%s ctx must be a pointer", name)
		} else if typeKind == reflect.Struct {
			input.type_ = &type_
			for i := 0; i < type_.NumField(); i++ {
				field := type_.Field(i)
				input, skip, err := c.checkFunctionInputStruct(&field)
				if skip {
					continue
				}
				if err != nil {
					return nil, "", fmt.Errorf("%s, type %s (#%d)", err.Error(), type_.Name(), i)
				}

				inFields[input.gqFieldName] = referToInput{
					inputIdx: iInList,
					input:    input,
				}
			}
		} else {
			return nil, "", fmt.Errorf("invalid struct item type %s (#%d)", type_.Name(), i)
		}

		ins = append(ins, input)
	}

	numOuts := t.NumOut()
	if numOuts == 0 {
		return nil, "", fmt.Errorf("%s no value returned", name)
	} else if numOuts > 2 {
		return nil, "", fmt.Errorf("%s cannot return more than 2 response values", name)
	}

	var outNr *int
	var outTypeObj *Obj
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

			outObj, err := c.check(outType)
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

	return &ObjMethod{
		isTypeMethod: isTypeMethod,
		ins:          ins,
		inFields:     inFields,
		outNr:        *outNr,
		outType:      *outTypeObj,
		errorOutNr:   hasErrorOut,
	}, formatGoNameToQL(trimmedName), nil
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

func valueLooksTrue(val string) bool {
	val = strings.ToLower(val)
	return val == "true" || val == "t" || val == "1"
}

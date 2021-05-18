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
	rootQuery       *Obj
	rootQueryValue  reflect.Value
	rootMethod      *Obj
	rootMethodValue reflect.Value
	m               sync.Mutex
	MaxDepth        uint8 // Default 255
}

type valueType int

const (
	valueTypeArray valueType = iota
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
	objContents map[string]*Obj

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

	isSpread bool
	ins      []Input

	outNr      int
	outType    Obj
	errorOutNr *int
}

type Input struct {
	isCtx bool // TODO add suport for this

	// Input type if isCtx is false
	type_ *reflect.Type
}

type SchemaOptions struct {
	noMethodEqualToQueryChecks bool // internal for testing
}

func ParseSchema(queries interface{}, methods interface{}, options SchemaOptions) (*Schema, error) {
	res := Schema{
		types:           Types{},
		rootQueryValue:  reflect.ValueOf(queries),
		rootMethodValue: reflect.ValueOf(methods),
		MaxDepth:        255,
	}

	obj, err := check(&res.types, reflect.TypeOf(queries))
	if err != nil {
		return nil, err
	}
	if obj.valueType != valueTypeObjRef {
		return nil, errors.New("Input queries must be a struct")
	}
	res.rootQuery = res.types[obj.typeName]

	obj, err = check(&res.types, reflect.TypeOf(methods))
	if err != nil {
		return nil, err
	}
	if obj.valueType != valueTypeObjRef {
		return nil, errors.New("Input methods must be a struct")
	}
	res.rootMethod = res.types[obj.typeName]

	if !options.noMethodEqualToQueryChecks {
		queryPkg := res.rootQuery.pkgPath + res.rootQuery.typeName
		methodPkg := res.rootMethod.pkgPath + res.rootMethod.typeName
		if queryPkg == methodPkg {
			return nil, errors.New("method and query cannot be the same struct")
		}
	}

	return &res, nil
}

func check(types *Types, t reflect.Type) (*Obj, error) {
	res := Obj{
		typeName: t.Name(),
		pkgPath:  t.PkgPath(),
	}

	switch t.Kind() {
	case reflect.Struct:
		res.valueType = valueTypeObj

		if res.typeName != "" {
			v, ok := types.Get(res.typeName)
			if ok {
				if v.pkgPath != res.pkgPath {
					return nil, fmt.Errorf("cannot have 2 structs with same type in structure: %s(%s) != %s(%s)", v.pkgPath, res.typeName, res.pkgPath, res.typeName)
				}

				res = v.getRef()
				return &res, nil
			}
		}

		res.objContents = map[string]*Obj{}

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
					methodObj, _, err := checkFunction(types, field.Name, field.Type, false)
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
					obj, err := check(types, field.Type)
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

		obj, err := check(types, t.Elem())
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
			methodObj, name, err := checkFunction(types, methodName, method.Type, true)
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

	if res.typeName != "" && res.valueType == valueTypeObj {
		res = types.Add(res)
	}

	return &res, nil
}

func checkFunction(types *Types, name string, t reflect.Type, isTypeMethod bool) (*ObjMethod, string, error) {
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

	isVariadic := t.IsVariadic()

	ins := []Input{}
	for i := 0; i < t.NumIn(); i++ {
		if i == 0 && isTypeMethod {
			// First argument can be skipped here
			continue
		}

		type_ := t.In(i)
		ins = append(ins, Input{type_: &type_})
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

			outObj, err := check(types, outType)
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
		isSpread:     isVariadic,
		ins:          ins,
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

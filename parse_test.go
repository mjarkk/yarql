package graphql

import (
	"reflect"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestValueLooksTrue(t *testing.T) {
	True(t, valueLooksTrue("true"))
	True(t, valueLooksTrue("t"))
	True(t, valueLooksTrue("1"))

	False(t, valueLooksTrue("false"))
	False(t, valueLooksTrue("f"))
	False(t, valueLooksTrue("0"))

	False(t, valueLooksTrue(""))
	False(t, valueLooksTrue("tr"))
	False(t, valueLooksTrue("tru"))
}

func TestFormatGoNameToQL(t *testing.T) {
	Equal(t, "input", formatGoNameToQL("input"))
	Equal(t, "input", formatGoNameToQL("Input"))
	Equal(t, "INPUT", formatGoNameToQL("INPUT"))
	Equal(t, "", formatGoNameToQL(""))
}

type TestCheckEmptyStructData struct{}

func TestCheckEmptyStruct(t *testing.T) {
	obj, err := check(&Types{}, reflect.TypeOf(TestCheckEmptyStructData{}))
	NoError(t, err)

	Equal(t, valueTypeObjRef, obj.valueType)
}

type TestCheckStructSimpleDemo struct {
	A string
	B int
	C float64
}

func TestCheckStructSimple(t *testing.T) {
	types := Types{}
	obj, err := check(&types, reflect.TypeOf(TestCheckStructSimpleDemo{}))
	NoError(t, err)

	Equal(t, obj.valueType, valueTypeObjRef)
	typeObj, ok := types[obj.typeName]
	True(t, ok)
	NotNil(t, typeObj.objContents)

	exists := map[string]reflect.Kind{
		"a": reflect.String,
		"b": reflect.Int,
		"c": reflect.Float64,
	}
	for name, expectedType := range exists {
		val, ok := typeObj.objContents[name]
		True(t, ok)
		Equal(t, valueTypeData, val.valueType)
		Equal(t, expectedType, val.dataValueType)
	}
}

func TestParseSchema(t *testing.T) {
	ParseSchema(TestCheckStructSimpleDemo{}, TestCheckStructSimpleDemo{}, SchemaOptions{noMethodEqualToQueryChecks: true})
}

type TestCheckStructWArrayData struct {
	Foo []string
}

func TestCheckStructWArray(t *testing.T) {
	types := Types{}
	ref, err := check(&types, reflect.TypeOf(TestCheckStructWArrayData{}))
	NoError(t, err)
	obj := types[ref.typeName]

	// Foo is an array
	val, ok := obj.objContents["foo"]
	True(t, ok)
	Equal(t, valueTypeArray, val.valueType)

	// Foo array content is correct
	val = val.innerContent
	NotNil(t, val)
	Equal(t, valueTypeData, val.valueType)
	Equal(t, reflect.String, val.dataValueType)
}

type TestCheckStructWPtrData struct {
	Foo *string
}

func TestCheckStructWPtr(t *testing.T) {
	types := Types{}
	ref, err := check(&types, reflect.TypeOf(TestCheckStructWPtrData{}))
	NoError(t, err)
	obj := types[ref.typeName]

	// Foo is a ptr
	val, ok := obj.objContents["foo"]
	True(t, ok)
	Equal(t, valueTypePtr, val.valueType)

	// Foo array content is correct
	val = val.innerContent
	NotNil(t, val)
	Equal(t, valueTypeData, val.valueType)
	Equal(t, reflect.String, val.dataValueType)
}

type TestCheckStructTagsData struct {
	Name        string `gqName:"otherName"`
	HiddenField string `gqIgnore:"true"`
}

func TestCheckStructTags(t *testing.T) {
	types := Types{}
	ref, err := check(&types, reflect.TypeOf(TestCheckStructTagsData{}))
	NoError(t, err)
	obj := types[ref.typeName]

	_, ok := obj.objContents["otherName"]
	True(t, ok, "name should now be called otherName")

	_, ok = obj.objContents["name"]
	False(t, ok, "name should now be called otherName and thus also not appear in the checkres")

	_, ok = obj.objContents["hiddenField"]
	False(t, ok, "hiddenField should be ignored")
}

func TestCheckInvalidStruct(t *testing.T) {
	_, err := check(&Types{}, reflect.TypeOf(struct {
		Foo interface{}
	}{}))
	Error(t, err)

	_, err = check(&Types{}, reflect.TypeOf(struct {
		Foo complex64
	}{}))
	Error(t, err)

	_, err = check(&Types{}, reflect.TypeOf(struct {
		Foo struct {
			Bar complex64
		}
	}{}))
	Error(t, err)
}

type TestCheckMethodsData struct{}

func (TestCheckMethodsData) ResolveName(in struct{}) string {
	return ""
}
func (TestCheckMethodsData) ResolveBanana(in struct{}) (string, error) {
	return "", nil
}
func (TestCheckMethodsData) ResolvePeer(in struct{}) string {
	return ""
}

func TestCheckMethods(t *testing.T) {
	types := Types{}
	ref, err := check(&types, reflect.TypeOf(TestCheckMethodsData{}))
	Nil(t, err)
	obj := types[ref.typeName]

	_, ok := obj.objContents["name"]
	True(t, ok)
	_, ok = obj.objContents["banana"]
	True(t, ok)
	_, ok = obj.objContents["peer"]
	True(t, ok)
}

type TestCheckMethodsFailData1 struct{}

func (TestCheckMethodsFailData1) ResolveName(in int) (string, string) {
	return "", ""
}

type TestCheckMethodsFailData2 struct{}

func (TestCheckMethodsFailData2) ResolveName(in int) (error, error) {
	return nil, nil
}

type TestCheckMethodsFailData3 struct{}

func (TestCheckMethodsFailData3) ResolveName(in int) func(string) string {
	return nil
}

func TestCheckMethodsFail(t *testing.T) {
	_, err := check(&Types{}, reflect.TypeOf(TestCheckMethodsFailData1{}))
	Error(t, err)

	_, err = check(&Types{}, reflect.TypeOf(TestCheckMethodsFailData2{}))
	Error(t, err)

	_, err = check(&Types{}, reflect.TypeOf(TestCheckMethodsFailData3{}))
	Error(t, err)
}

type TestCheckStructFuncsData struct {
	Name func(struct{}) string
}

func TestCheckStructFuncs(t *testing.T) {
	types := Types{}
	ref, err := check(&types, reflect.TypeOf(TestCheckStructFuncsData{}))
	Nil(t, err)
	obj := types[ref.typeName]

	_, ok := obj.objContents["name"]
	True(t, ok)
}

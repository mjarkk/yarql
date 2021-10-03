package graphql

import (
	"reflect"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestFormatGoNameToQL(t *testing.T) {
	Equal(t, "input", formatGoNameToQL("input"))
	Equal(t, "input", formatGoNameToQL("Input"))
	Equal(t, "INPUT", formatGoNameToQL("INPUT"))
	Equal(t, "", formatGoNameToQL(""))
}

type TestCheckEmptyStructData struct{}

func newParseCtx() *parseCtx {
	return &parseCtx{
		schema:        NewSchema(),
		parsedMethods: []*objMethod{},
	}
}

func TestCheckEmptyStruct(t *testing.T) {
	obj, err := newParseCtx().check(reflect.TypeOf(TestCheckEmptyStructData{}), false)
	NoError(t, err)

	Equal(t, valueTypeObjRef, obj.valueType)
}

type TestCheckStructSimpleDemo struct {
	A string
	B int
	C float64
}

func TestCheckStructSimple(t *testing.T) {
	ctx := newParseCtx()
	obj, err := ctx.check(reflect.TypeOf(TestCheckStructSimpleDemo{}), false)
	NoError(t, err)

	Equal(t, obj.valueType, valueTypeObjRef)
	typeObj, ok := ctx.schema.types[obj.typeName]
	True(t, ok)
	NotNil(t, typeObj.objContents)

	exists := map[string]reflect.Kind{
		"a": reflect.String,
		"b": reflect.Int,
		"c": reflect.Float64,
	}
	for name, expectedType := range exists {
		val, ok := typeObj.objContents[getObjKey([]byte(name))]
		True(t, ok)
		Equal(t, valueTypeData, val.valueType)
		Equal(t, expectedType, val.dataValueType)
	}
}

func TestParseSchema(t *testing.T) {
	NewSchema().Parse(TestCheckStructSimpleDemo{}, TestCheckStructSimpleDemo{}, &SchemaOptions{noMethodEqualToQueryChecks: true})
}

type TestCheckStructWArrayData struct {
	Foo []string
}

func TestCheckStructWArray(t *testing.T) {
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckStructWArrayData{}), false)
	NoError(t, err)
	obj := ctx.schema.types[ref.typeName]

	// Foo is an array
	val, ok := obj.objContents[getObjKey([]byte("foo"))]
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
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckStructWPtrData{}), false)
	NoError(t, err)
	obj := ctx.schema.types[ref.typeName]

	// Foo is a ptr
	val, ok := obj.objContents[getObjKey([]byte("foo"))]
	True(t, ok)
	Equal(t, valueTypePtr, val.valueType)

	// Foo array content is correct
	val = val.innerContent
	NotNil(t, val)
	Equal(t, valueTypeData, val.valueType)
	Equal(t, reflect.String, val.dataValueType)
}

type TestCheckStructTagsData struct {
	Name        string `gq:"otherName"`
	HiddenField string `gq:"-"`
}

func TestCheckStructTags(t *testing.T) {
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckStructTagsData{}), false)
	NoError(t, err)
	obj := ctx.schema.types[ref.typeName]

	_, ok := obj.objContents[getObjKey([]byte("otherName"))]
	True(t, ok, "name should now be called otherName")

	_, ok = obj.objContents[getObjKey([]byte("name"))]
	False(t, ok, "name should now be called otherName and thus also not appear in the checkres")

	_, ok = obj.objContents[getObjKey([]byte("hiddenField"))]
	False(t, ok, "hiddenField should be ignored")
}

func TestCheckInvalidStruct(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(struct {
		Foo interface{}
	}{}), false)
	Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(struct {
		Foo complex64
	}{}), false)
	Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(struct {
		Foo struct {
			Bar complex64
		}
	}{}), false)
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
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckMethodsData{}), false)
	Nil(t, err)
	obj := ctx.schema.types[ref.typeName]

	_, ok := obj.objContents[getObjKey([]byte("name"))]
	True(t, ok)
	_, ok = obj.objContents[getObjKey([]byte("banana"))]
	True(t, ok)
	_, ok = obj.objContents[getObjKey([]byte("peer"))]
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
	_, err := newParseCtx().check(reflect.TypeOf(TestCheckMethodsFailData1{}), false)
	Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(TestCheckMethodsFailData2{}), false)
	Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(TestCheckMethodsFailData3{}), false)
	Error(t, err)
}

type TestCheckStructFuncsData struct {
	Name func(struct{}) string
}

func TestCheckStructFuncs(t *testing.T) {
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckStructFuncsData{}), false)
	Nil(t, err)
	obj := ctx.schema.types[ref.typeName]

	_, ok := obj.objContents[getObjKey([]byte("name"))]
	True(t, ok)
}

type ReferToSelf1 struct {
	Bar *ReferToSelf1
}

func TestReferenceLoop1(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(ReferToSelf1{}), false)
	Nil(t, err)
}

type ReferToSelf2 struct {
	Bar []ReferToSelf1
}

func TestReferenceLoop2(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(ReferToSelf2{}), false)
	Nil(t, err)
}

type ReferToSelf3 struct {
	Bar func() ReferToSelf1
}

func TestReferenceLoop3(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(ReferToSelf3{}), false)
	Nil(t, err)
}

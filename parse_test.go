package graphql

import (
	"reflect"
	"testing"

	a "github.com/stretchr/testify/assert"
)

func TestFormatGoNameToQL(t *testing.T) {
	a.Equal(t, "input", formatGoNameToQL("input"))
	a.Equal(t, "input", formatGoNameToQL("Input"))
	a.Equal(t, "INPUT", formatGoNameToQL("INPUT"))
	a.Equal(t, "", formatGoNameToQL(""))
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
	a.NoError(t, err)

	a.Equal(t, valueTypeObjRef, obj.valueType)
}

type TestCheckStructSimpleDemo struct {
	A string
	B int
	C float64
}

func TestCheckStructSimple(t *testing.T) {
	ctx := newParseCtx()
	obj, err := ctx.check(reflect.TypeOf(TestCheckStructSimpleDemo{}), false)
	a.NoError(t, err)

	a.Equal(t, obj.valueType, valueTypeObjRef)
	typeObj, ok := ctx.schema.types[obj.typeName]
	a.True(t, ok)
	a.NotNil(t, typeObj.objContents)

	exists := map[string]reflect.Kind{
		"a": reflect.String,
		"b": reflect.Int,
		"c": reflect.Float64,
	}
	for name, expectedType := range exists {
		val, ok := typeObj.objContents[getObjKey([]byte(name))]
		a.True(t, ok)
		a.Equal(t, valueTypeData, val.valueType)
		a.Equal(t, expectedType, val.dataValueType)
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
	a.NoError(t, err)
	obj := ctx.schema.types[ref.typeName]

	// Foo is an array
	val, ok := obj.objContents[getObjKey([]byte("foo"))]
	a.True(t, ok)
	a.Equal(t, valueTypeArray, val.valueType)

	// Foo array content is correct
	val = val.innerContent
	a.NotNil(t, val)
	a.Equal(t, valueTypeData, val.valueType)
	a.Equal(t, reflect.String, val.dataValueType)
}

type TestCheckStructWPtrData struct {
	Foo *string
}

func TestCheckStructWPtr(t *testing.T) {
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckStructWPtrData{}), false)
	a.NoError(t, err)
	obj := ctx.schema.types[ref.typeName]

	// Foo is a ptr
	val, ok := obj.objContents[getObjKey([]byte("foo"))]
	a.True(t, ok)
	a.Equal(t, valueTypePtr, val.valueType)

	// Foo array content is correct
	val = val.innerContent
	a.NotNil(t, val)
	a.Equal(t, valueTypeData, val.valueType)
	a.Equal(t, reflect.String, val.dataValueType)
}

type TestCheckStructTagsData struct {
	Name        string `gq:"otherName"`
	HiddenField string `gq:"-"`
}

func TestCheckStructTags(t *testing.T) {
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckStructTagsData{}), false)
	a.NoError(t, err)
	obj := ctx.schema.types[ref.typeName]

	_, ok := obj.objContents[getObjKey([]byte("otherName"))]
	a.True(t, ok, "name should now be called otherName")

	_, ok = obj.objContents[getObjKey([]byte("name"))]
	a.False(t, ok, "name should now be called otherName and thus also not appear in the checkres")

	_, ok = obj.objContents[getObjKey([]byte("hiddenField"))]
	a.False(t, ok, "hiddenField should be ignored")
}

func TestCheckInvalidStruct(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(struct {
		Foo interface{}
	}{}), false)
	a.Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(struct {
		Foo complex64
	}{}), false)
	a.Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(struct {
		Foo struct {
			Bar complex64
		}
	}{}), false)
	a.Error(t, err)
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
func (TestCheckMethodsData) ResolveId(in struct{}) (int, AttrIsID) {
	return 0, 0
}

func TestCheckMethods(t *testing.T) {
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckMethodsData{}), false)
	a.Nil(t, err)
	obj := ctx.schema.types[ref.typeName]

	field, ok := obj.objContents[getObjKey([]byte("name"))]
	a.True(t, ok)
	a.False(t, field.isID)
	a.Nil(t, field.method.errorOutNr)

	field, ok = obj.objContents[getObjKey([]byte("banana"))]
	a.True(t, ok)
	a.False(t, field.isID)
	a.NotNil(t, field.method.errorOutNr)

	field, ok = obj.objContents[getObjKey([]byte("peer"))]
	a.True(t, ok)
	a.False(t, field.isID)
	a.Nil(t, field.method.errorOutNr)

	field, ok = obj.objContents[getObjKey([]byte("id"))]
	a.True(t, ok)
	a.True(t, field.isID)
	a.Nil(t, field.method.errorOutNr)
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
	a.Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(TestCheckMethodsFailData2{}), false)
	a.Error(t, err)

	_, err = newParseCtx().check(reflect.TypeOf(TestCheckMethodsFailData3{}), false)
	a.Error(t, err)
}

type TestCheckStructFuncsData struct {
	Name func(struct{}) string
}

func TestCheckStructFuncs(t *testing.T) {
	ctx := newParseCtx()
	ref, err := ctx.check(reflect.TypeOf(TestCheckStructFuncsData{}), false)
	a.Nil(t, err)
	obj := ctx.schema.types[ref.typeName]

	_, ok := obj.objContents[getObjKey([]byte("name"))]
	a.True(t, ok)
}

type ReferToSelf1 struct {
	Bar *ReferToSelf1
}

func TestReferenceLoop1(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(ReferToSelf1{}), false)
	a.Nil(t, err)
}

type ReferToSelf2 struct {
	Bar []ReferToSelf1
}

func TestReferenceLoop2(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(ReferToSelf2{}), false)
	a.Nil(t, err)
}

type ReferToSelf3 struct {
	Bar func() ReferToSelf1
}

func TestReferenceLoop3(t *testing.T) {
	_, err := newParseCtx().check(reflect.TypeOf(ReferToSelf3{}), false)
	a.Nil(t, err)
}

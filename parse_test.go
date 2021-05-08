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
}

func TestCheckString(t *testing.T) {
	obj, err := check(reflect.TypeOf(""))
	NoError(t, err)

	Equal(t, valueTypeData, obj.valueType)
	Equal(t, reflect.String, obj.dataValueType)
}

type TestCheckStructSimpleDemo struct {
	A string
	B int
	C float64
}

func TestCheckStructSimple(t *testing.T) {
	obj, err := check(reflect.TypeOf(TestCheckStructSimpleDemo{}))
	NoError(t, err)

	Equal(t, obj.valueType, valueTypeObj)
	NotNil(t, obj.objContents)

	exists := map[string]reflect.Kind{
		"a": reflect.String,
		"b": reflect.Int,
		"c": reflect.Float64,
	}
	for name, expectedType := range exists {
		val, ok := obj.objContents[name]
		True(t, ok)
		Equal(t, valueTypeData, val.valueType)
		Equal(t, expectedType, val.dataValueType)
	}
}

func TestParseSchema(t *testing.T) {
	ParseSchema(map[string]interface{}{
		"a": TestCheckStructSimpleDemo{},
	})
}

type TestCheckStructWArrayData struct {
	Foo []string
}

func TestCheckStructWArray(t *testing.T) {
	obj, err := check(reflect.TypeOf(TestCheckStructWArrayData{}))
	NoError(t, err)

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
	obj, err := check(reflect.TypeOf(TestCheckStructWPtrData{}))
	NoError(t, err)

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
	obj, err := check(reflect.TypeOf(TestCheckStructTagsData{}))
	NoError(t, err)

	_, ok := obj.objContents["otherName"]
	True(t, ok, "name should now be called otherName")

	_, ok = obj.objContents["name"]
	False(t, ok, "name should now be called otherName and thus also not appear in the checkres")

	_, ok = obj.objContents["hiddenField"]
	False(t, ok, "hiddenField should be ignored")
}

func TestCheckInvalidStruct(t *testing.T) {
	_, err := check(reflect.TypeOf(struct {
		Foo interface{}
	}{}))
	Error(t, err)

	_, err = check(reflect.TypeOf(struct {
		Foo complex64
	}{}))
	Error(t, err)

	_, err = check(reflect.TypeOf(struct {
		Foo struct {
			Bar complex64
		}
	}{}))
	Error(t, err)
}

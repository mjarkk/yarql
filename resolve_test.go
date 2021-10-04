package graphql

import (
	"bytes"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"reflect"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestValueToJson(t *testing.T) {
	string_ := string(`a"b`)
	boolTrue := bool(true)
	boolFalse := bool(false)
	int_ := int(1)
	int8_ := int8(2)
	int16_ := int16(3)
	int32_ := int32(4)
	int64_ := int64(5)
	uint_ := uint(6)
	uint8_ := uint8(7)
	uint16_ := uint16(8)
	uint32_ := uint32(9)
	uint64_ := uint64(10)
	uintptr_ := uintptr(11)
	float32_ := float32(12)
	float64_ := float64(13)
	float64WExponent := 100e-100

	var stringPtr *string
	var boolPtr *bool
	var intPtr *int
	var int8Ptr *int8
	var int16Ptr *int16
	var int32Ptr *int32
	var int64Ptr *int64
	var uintPtr *uint
	var uint8Ptr *uint8
	var uint16Ptr *uint16
	var uint32Ptr *uint32
	var uint64Ptr *uint64
	var uintptrPtr *uintptr
	var float32Ptr *float32
	var float64Ptr *float64

	options := []struct {
		value  interface{}
		expect string
	}{
		{string_, `"a\"b"`},
		{boolTrue, "true"},
		{boolFalse, "false"},
		{int_, "1"},
		{int8_, "2"},
		{int16_, "3"},
		{int32_, "4"},
		{int64_, "5"},
		{uint_, "6"},
		{uint8_, "7"},
		{uint16_, "8"},
		{uint32_, "9"},
		{uint64_, "10"},
		{uintptr_, "null"}, // We do not support this datavalue
		{float32_, "12"},
		{float64_, "13"},
		{float64WExponent, "1e-98"},

		{&string_, `"a\"b"`},
		{&boolTrue, "true"},
		{&boolFalse, "false"},
		{&int_, "1"},
		{&int8_, "2"},
		{&int16_, "3"},
		{&int32_, "4"},
		{&int64_, "5"},
		{&uint_, "6"},
		{&uint8_, "7"},
		{&uint16_, "8"},
		{&uint32_, "9"},
		{&uint64_, "10"},
		{&uintptr_, "null"}, // We do not support this datavalue
		{&float32_, "12"},
		{&float64_, "13"},

		{stringPtr, `null`},
		{boolPtr, "null"},
		{intPtr, "null"},
		{int8Ptr, "null"},
		{int16Ptr, "null"},
		{int32Ptr, "null"},
		{int64Ptr, "null"},
		{uintPtr, "null"},
		{uint8Ptr, "null"},
		{uint16Ptr, "null"},
		{uint32Ptr, "null"},
		{uint64Ptr, "null"},
		{uintptrPtr, "null"},
		{float32Ptr, "null"},
		{float64Ptr, "null"},

		{complex64(1), "null"},
	}
	for _, option := range options {
		c := &Ctx{result: []byte{}}
		v := reflect.ValueOf(option.value)
		c.valueToJson(v, v.Kind())
		Equal(t, option.expect, string(c.result))
	}
}

type TestResolveMaxDeptData struct {
	Foo struct {
		Bar struct {
			Baz struct {
				FooBar struct {
					BarBaz struct {
						BazFoo string
					}
				}
			}
		}
	}
}

func TestExecMaxDept(t *testing.T) {
	out, errs := parseAndTestWithOptions(t, NewSchema(), `{foo{bar{baz{fooBar{barBaz{bazFoo}}}}}}`, TestExecMaxDeptData{}, M{}, 3, ResolveOptions{})
	Greater(t, len(errs), 0)
	Equal(t, `{"data":{"foo":{"bar":{"baz":null}}},"errors":[{"message":"reached max dept","path":["foo","bar","baz"]}]}`, out)
}

type TestResolveStructTypeMethodWithCtxData struct{}

func (TestResolveStructTypeMethodWithCtxData) ResolveBar(c *BytecodeCtx) TestResolveStructTypeMethodWithCtxDataInner {
	c.SetValue("baz", "bar")
	return TestResolveStructTypeMethodWithCtxDataInner{}
}

type TestResolveStructTypeMethodWithCtxDataInner struct{}

func (TestResolveStructTypeMethodWithCtxDataInner) ResolveFoo(c *BytecodeCtx) string {
	return c.GetValue("baz").(string)
}

func (TestResolveStructTypeMethodWithCtxData) ResolveBaz(c *BytecodeCtx) (string, error) {
	value, ok := c.GetValueOk("baz")
	if !ok {
		return "", errors.New("baz not set by bar resolver")
	}
	return value.(string), nil
}

type TestResolveWithContextData struct{}

func (TestResolveWithContextData) ResolveFoo(ctx *BytecodeCtx) string {
	<-ctx.context.Done()
	return "Oh no the time has ran out"
}

type TestResolveWithPreDefinedVarsData struct{}

func (TestResolveWithPreDefinedVarsData) ResolveFoo(ctx *BytecodeCtx) string {
	return ctx.GetValue("bar").(string)
}

type TestResolveWithFileData struct{}

func (TestResolveWithFileData) ResolveFoo(args struct{ File *multipart.FileHeader }) string {
	if args.File == nil {
		return ""
	}
	f, err := args.File.Open()
	if err != nil {
		return ""
	}
	defer f.Close()
	fileContents, err := ioutil.ReadAll(f)
	if err != nil {
		return ""
	}
	return string(fileContents)
}

func TestExecWithFile(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	multiPartWriter := multipart.NewWriter(buf)
	writer, err := multiPartWriter.CreateFormFile("FILE_ID", "test.txt")
	if err != nil {
		panic(err)
	}
	writer.Write([]byte("hello world"))
	boundary := multiPartWriter.Boundary()
	err = multiPartWriter.Close()
	if err != nil {
		panic(err)
	}

	multiPartReader := multipart.NewReader(buf, boundary)
	form, err := multiPartReader.ReadForm(1024 * 1024)
	if err != nil {
		panic(err)
	}

	out, errs := parseAndTestWithOptions(t, NewSchema(), `{foo(file: "FILE_ID")}`, TestResolveWithFileData{}, M{}, 255, ResolveOptions{
		GetFormFile: func(key string) (*multipart.FileHeader, error) {
			f, ok := form.File[key]
			if !ok || len(f) == 0 {
				return nil, nil
			}
			return f[0], nil
		},
	})
	for _, err := range errs {
		panic(err)
	}
	Equal(t, `{"data":{"foo":"hello world"}}`, out)
}

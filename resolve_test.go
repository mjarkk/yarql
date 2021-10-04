package graphql

import (
	"bytes"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"testing"

	. "github.com/stretchr/testify/assert"
)

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

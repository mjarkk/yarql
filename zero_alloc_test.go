package graphql

import (
	"strings"
	"testing"

	. "github.com/stretchr/testify/assert"
)

type TestPathStaysCorrectData struct {
	Bar    TestPathStaysCorrectDataBar
	Foo    []TestPathStaysCorrectDataFoo
	Baz    TestPathStaysCorrectDataBar
	FooBar []TestPathStaysCorrectDataBar
}

func (TestPathStaysCorrectData) ResolvePath(c *Ctx) string {
	return c.Path()
}

type TestPathStaysCorrectDataFoo struct {
	Bar TestPathStaysCorrectDataBar
}

func (TestPathStaysCorrectDataFoo) ResolvePath(c *Ctx) string {
	return c.Path()
}

type TestPathStaysCorrectDataBar struct {
	Foo []TestPathStaysCorrectDataFoo
}

func (TestPathStaysCorrectDataBar) ResolvePath(c *Ctx) string {
	return c.Path()
}

func TestPathStaysCorrect(t *testing.T) {
	queryType := TestPathStaysCorrectData{
		Bar: TestPathStaysCorrectDataBar{
			Foo: []TestPathStaysCorrectDataFoo{
				{Bar: TestPathStaysCorrectDataBar{}},
				{Bar: TestPathStaysCorrectDataBar{}},
			},
		},
		Foo: []TestPathStaysCorrectDataFoo{
			{Bar: TestPathStaysCorrectDataBar{}},
			{Bar: TestPathStaysCorrectDataBar{}},
		},
		Baz: TestPathStaysCorrectDataBar{
			Foo: []TestPathStaysCorrectDataFoo{
				{Bar: TestPathStaysCorrectDataBar{}},
				{Bar: TestPathStaysCorrectDataBar{}},
			},
		},
		FooBar: []TestPathStaysCorrectDataBar{
			{Foo: []TestPathStaysCorrectDataFoo{}},
			{Foo: []TestPathStaysCorrectDataFoo{}},
		},
	}

	query := `{
		path
		bar {
			path
			foo {
				path
				bar {
					path
					foo
				}
			}
		}
		foo {
			path
			bar {
				path
				foo
			}
		}
		baz {
			path
			foo {
				path
				bar {
					path
					foo
				}
			}
		}
		fooBar {
			path
			foo {
				path
				foo {
					path
					bar
				}
			}
		}
	}`

	out, errs := parseAndTest(t, query, queryType, M{})
	for _, err := range errs {
		panic(err)
	}

	expectedOut := `{
		"path": "[
			\"path\"
		]",
		"bar": {
			"path": "[
				\"bar\",
				\"path\"
			]",
			"foo": [
				{
					"path": "[
						\"bar\",
						\"foo\",
						0,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"bar\",
							\"foo\",
							0,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				},
				{
					"path": "[
						\"bar\",
						\"foo\",
						1,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"bar\",
							\"foo\",
							1,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				}
			]
		},
		"foo": [
			{
				"path": "[
					\"foo\",
					0,
					\"path\"
				]",
				"bar": {
					"path": "[
						\"foo\",
						0,
						\"bar\",
						\"path\"
					]",
					"foo": null
				}
			},
			{
				"path": "[
					\"foo\",
					1,
					\"path\"
				]",
				"bar": {
					"path": "[
						\"foo\",
						1,
						\"bar\",
						\"path\"
					]",
					"foo": null
				}
			}
		],
		"baz": {
			"path": "[
				\"baz\",
				\"path\"
			]",
			"foo": [
				{
					"path": "[
						\"baz\",
						\"foo\",
						0,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"baz\",
							\"foo\",
							0,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				},
				{
					"path": "[
						\"baz\",
						\"foo\",
						1,
						\"path\"
					]",
					"bar": {
						"path": "[
							\"baz\",
							\"foo\",
							1,
							\"bar\",
							\"path\"
						]",
						"foo": null
					}
				}
			]
		},
		"fooBar": [
			{
				"path": "[
					\"fooBar\",
					0,
					\"path\"
				]",
				"foo": []
			},
			{
				"path": "[
					\"fooBar\",
					1,
					\"path\"
				]",
				"foo": []
			}
		]
	}`
	expectedOut = strings.ReplaceAll(expectedOut, " ", "")
	expectedOut = strings.ReplaceAll(expectedOut, "\n", "")
	expectedOut = strings.ReplaceAll(expectedOut, "\t", "")
	Equal(t, expectedOut, out)
}

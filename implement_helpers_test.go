package graphql

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	. "github.com/stretchr/testify/assert"
)

func checkValidJson(t *testing.T, in string) {
	True(t, json.Valid([]byte(in)), in)
}

func TestGenerateResponse(t *testing.T) {
	checkValidJson(t, GenerateResponse("{}", nil))
}

func TestGenerateResponseWError(t *testing.T) {
	checkValidJson(t, GenerateResponse("{}", []error{errors.New("test")}))
}

func TestGenerateResponseWErrors(t *testing.T) {
	checkValidJson(t, GenerateResponse("{}", []error{errors.New("A"), errors.New("B")}))
}

func TestGenerateResponseWErrorWPath(t *testing.T) {
	j := GenerateResponse("{}", []error{ErrorWPath{err: errors.New("name invalid"), path: []string{`"users"`, `2`, `"name"`}}})
	checkValidJson(t, j)
	True(t, strings.Contains(j, `"users"`))
	True(t, strings.Contains(j, `2`))
	True(t, strings.Contains(j, `"name"`))
}

func TestGenerateResponseWErrorWLocation(t *testing.T) {
	j := GenerateResponse("{}", []error{ErrorWLocation{err: errors.New("name invalid"), line: 4, column: 9}})
	checkValidJson(t, j)
	True(t, strings.Contains(j, "4"), "Contains the line number")
	True(t, strings.Contains(j, "9"), "Contains the column number")
}

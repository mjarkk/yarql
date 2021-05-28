package graphql

import (
	"log"
	"reflect"
	"strings"
)

var renamedTypes = map[string]string{}

func TypeRename(type_ interface{}, newName string) string {
	t := reflect.TypeOf(type_)
	originalName := t.Name()

	if originalName == "" {
		log.Panicf("GrahpQl Can only rename struct type with type name\n")
	}
	if t.Kind() != reflect.Struct {
		log.Panicf("GrahpQl Cannot rename type of %s with name: %s and package: %s, can only rename Structs\n", t.Kind().String(), originalName, t.PkgPath())
	}

	newName = strings.TrimSpace(newName)
	if len(newName) == 0 {
		log.Panicf("GrahpQl cannot rename to empty string on type: %s %s\n", t.PkgPath(), originalName)
	}

	renamedTypes[originalName] = newName

	return newName
}

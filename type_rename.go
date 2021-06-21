package graphql

import (
	"log"
	"reflect"
	"strings"
)

var renamedTypes = map[string]string{}

// TypeRename renames the graphql type of the input type
// By default the typename of the struct is used but you might want to change this form time to time and with this you can
func TypeRename(type_ interface{}, newName string, force ...bool) string {
	t := reflect.TypeOf(type_)
	originalName := t.Name()

	if originalName == "" {
		log.Panicf("GraphQl Can only rename struct type with type name\n")
	}
	if t.Kind() != reflect.Struct {
		log.Panicf("GraphQl Cannot rename type of %s with name: %s and package: %s, can only rename Structs\n", t.Kind().String(), originalName, t.PkgPath())
	}

	newName = strings.TrimSpace(newName)
	if len(newName) == 0 {
		log.Panicf("GraphQl cannot rename to empty string on type: %s %s\n", t.PkgPath(), originalName)
	}

	if len(force) == 0 || !force[0] {
		err := validGraphQlName(newName)
		if err != nil {
			log.Panicf("GraphQl cannot rename typeof of %s with name %s to %s, err: %s", t.Kind().String(), originalName, newName, err.Error())
		}
	}

	renamedTypes[originalName] = newName

	return newName
}

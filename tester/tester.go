package tester

import (
	"encoding/json"
	"errors"
	"fmt"

	graphql "github.com/mjarkk/yarql"
)

// Type contains a subset of fields from the graphql __Type type
// https://spec.graphql.org/October2021/#sec-Schema-Introspection
type Type struct {
	Kind   string  `json:"kind"`
	Fields []Field `json:"fields"`
}

// Field contains a subset of fields from the graphql __Field type
// https://spec.graphql.org/October2021/#sec-Schema-Introspection
type Field struct {
	Name string `json:"name"`
}

// GetTypeByName returns a schema type based on it's typename
func GetTypeByName(s *graphql.Schema, typename string) *Type {
	vars := map[string]string{"typename": typename}
	varsJSON, _ := json.Marshal(vars)

	query := `query ($typename: String) {
		__type(name: $typename) {
			kind
			fields {
				name
			}
		}
	}`
	errs := s.Resolve([]byte(query), graphql.ResolveOptions{
		NoMeta:    true,
		Variables: string(varsJSON),
	})
	if len(errs) != 0 {
		return nil
	}

	type Res struct {
		Type *Type `json:"__type"`
	}

	var res Res
	err := json.Unmarshal(s.Result, &res)
	if err != nil {
		return nil
	}

	return res.Type
}

// HasType returns true if the typename exists on the schema
func HasType(s *graphql.Schema, typename string) bool {
	return GetTypeByName(s, typename) != nil
}

// TypeKind returns the kind of a type
// If no type is found an empty string is returned
func TypeKind(s *graphql.Schema, typename string) string {
	qlType := GetTypeByName(s, typename)
	if qlType == nil {
		return ""
	}
	return qlType.Kind
}

var (
	errTypeNotFound = errors.New("type not found")
)

// HasFields checks weather the type has the specified fields
func HasFields(s *graphql.Schema, typename string, fields []string) error {
	qlType := GetTypeByName(s, typename)
	if qlType == nil {
		return errTypeNotFound
	}

	if qlType.Kind != "OBJECT" && qlType.Kind != "INTERFACE" {
		return errors.New("type must be a OBJECT or INTERFACE")
	}

	for _, field := range fields {
		found := false
		for _, qlField := range qlType.Fields {
			if qlField.Name == field {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("field %s not found on type %s", field, typename)
		}
	}

	return nil
}

// OnlyHasFields checks weather the type only has the specified fields and no more
func OnlyHasFields(s *graphql.Schema, typename string, fields []string) error {
	qlType := GetTypeByName(s, typename)
	if qlType == nil {
		return errTypeNotFound
	}

	if qlType.Kind != "OBJECT" && qlType.Kind != "INTERFACE" {
		return errors.New("type must be a OBJECT or INTERFACE")
	}

	for _, field := range fields {
		found := false
		for _, qlField := range qlType.Fields {
			if qlField.Name == field {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("field %s not found on type %s", field, typename)
		}
	}

	for _, qlField := range qlType.Fields {
		found := false
		for _, field := range fields {
			if qlField.Name == field {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("found extra field with name %s on type %s", qlField.Name, typename)
		}
	}

	return nil
}

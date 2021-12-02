package graphql

import (
	"errors"
	"reflect"
)

// DirectiveLocation defines the location a directive can be used in
type DirectiveLocation uint8

const (
	// DirectiveLocationField can be called from a field
	DirectiveLocationField DirectiveLocation = iota
	// DirectiveLocationFragment can be called from a fragment
	DirectiveLocationFragment
	// DirectiveLocationFragmentInline can be called from a inline fragment
	DirectiveLocationFragmentInline
)

// String returns the DirectiveLocation as a string
func (l DirectiveLocation) String() string {
	switch l {
	case DirectiveLocationField:
		return "<DirectiveLocationField>"
	case DirectiveLocationFragment:
		return "<DirectiveLocationFragment>"
	case DirectiveLocationFragmentInline:
		return "<DirectiveLocationFragmentInline>"
	default:
		return "<UNKNOWN DIRECTIVE LOCATION>"
	}
}

// ToQlDirectiveLocation returns the matching graphql location
func (l DirectiveLocation) ToQlDirectiveLocation() __DirectiveLocation {
	switch l {
	case DirectiveLocationField:
		return directiveLocationField
	case DirectiveLocationFragment:
		return directiveLocationFragmentSpread
	case DirectiveLocationFragmentInline:
		return directiveLocationInlineFragment
	default:
		return directiveLocationField
	}
}

// Directive is what defines a directive
type Directive struct {
	// Required
	Name  string
	Where []DirectiveLocation
	// Should be of type: func(args like any other method) DirectiveModifier
	Method           interface{}
	methodReflection reflect.Value
	parsedMethod     *objMethod

	// Not required
	Description string
}

// TODO
// type ModifyOnWriteContent func(bytes []byte) []byte

// DirectiveModifier defines modifications to the response
// Nothing is this struct is required and will be ignored if not set
type DirectiveModifier struct {
	// Skip field/(inline)fragment
	Skip bool

	// TODO make this
	// ModifyOnWriteContent allows you to modify field JSON response data before it's written to the result
	// Note that there is no checking for validation here it's up to you to return valid json
	// ModifyOnWriteContent ModifyOnWriteContent
}

// RegisterDirective registers a new directive
func (s *Schema) RegisterDirective(directive Directive) error {
	if s.parsed {
		return errors.New("(*graphql.Schema).RegisterDirective() cannot be ran after (*graphql.Schema).Parse()")
	}

	err := checkDirective(&directive)
	if err != nil {
		return err
	}

	ptrToDirective := &directive
	for _, location := range directive.Where {
		directivesForLocation, ok := s.definedDirectives[location]
		if !ok {
			directivesForLocation = []*Directive{}
		} else {
			// Check for already defined directives with the same name
			for _, alreadyDefinedDirective := range directivesForLocation {
				if directive.Name == alreadyDefinedDirective.Name {
					return errors.New("you cannot have duplicated directive names in " + location.String() + " with name " + directive.Name)
				}
			}
		}
		directivesForLocation = append(directivesForLocation, ptrToDirective)
		s.definedDirectives[location] = directivesForLocation
	}

	return nil
}

func checkDirective(directive *Directive) error {
	if len(directive.Name) == 0 {
		return errors.New("cannot register directive with empty name")
	}
	for _, char := range directive.Name {
		if char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char == '_' {
			continue
		}
		return errors.New(string(char) + " in " + directive.Name + " is not allowed as directive name")
	}
	if directive.Where == nil {
		return errors.New("where must be defined")
	}
	if directive.Method == nil {
		return errors.New("method must be defined")
	}
	if directive.Method == nil {
		return errors.New("method must be defined")
	}
	directive.methodReflection = reflect.ValueOf(directive.Method)
	if directive.methodReflection.IsNil() {
		return errors.New("method must be defined")
	}
	if directive.methodReflection.Kind() != reflect.Func {
		return errors.New("method is not a function")
	}
	methodType := directive.methodReflection.Type()
	switch methodType.NumOut() {
	case 0:
		return errors.New("method should return DirectiveModifier")
	case 1:
		// OK
	default:
		return errors.New("method should only return DirectiveModifier")
	}

	outType := methodType.Out(0)
	directiveModifierType := reflect.TypeOf(DirectiveModifier{})
	if outType.Name() != directiveModifierType.Name() || outType.PkgPath() != directiveModifierType.PkgPath() {
		return errors.New("method should return DirectiveModifier")
	}

	directive.parsedMethod = &objMethod{
		isTypeMethod: false,
		goType:       methodType,

		ins:        []baseInput{},
		inFields:   map[string]referToInput{},
		checkedIns: false,
	}

	// Inputs checked in (s *Schema).Parse(..)

	return nil
}

package graphql

import "reflect"

type DirectiveLocation uint8

const (
	// The directive can be called from a field
	DirectiveLocationField DirectiveLocation = iota
	// The directive can be called from a fragment
	DirectiveLocationFragment
	// The directive can be called from a inline fragment
	DirectiveLocationFragmentInline
)

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

type Directive struct {
	// Required
	Name  string
	Where []DirectiveLocation
	// Should be of type: func(args like any other method) DirectiveModdifier
	Method interface{}

	// Not required
	Description string
}

// DirectiveModdifier defines modifications to the response
// Nothing is this struct is required and will be ignored if not set
type DirectiveModdifier struct {
	// Skip field/(inline)fragment
	Skip bool

	// ModifyOnWriteContent allows you to modify field JSON response data before it's written to the result
	// Note that there is no checking for validation here it's up to you to return valid json
	ModifyOnWriteContent func(bytes []byte) []byte
}

var definedDirectives = map[DirectiveLocation][]Directive{}

func RegisterDirective(directive Directive) bool {
	checkDirective(&directive)
	for _, location := range directive.Where {
		directivesForLocation, ok := definedDirectives[location]
		if !ok {
			directivesForLocation = []Directive{}
		} else {
			// Check for already defined directives with the same name
			for _, alreadyDefinedDirective := range directivesForLocation {
				if directive.Name == alreadyDefinedDirective.Name {
					panic("you cannot have duplicated directive names in " + location.String() + " with name " + directive.Name)
				}
			}
		}
		directivesForLocation = append(directivesForLocation, directive)
		definedDirectives[location] = directivesForLocation
	}
	return true
}

func checkDirective(directive *Directive) {
	if len(directive.Name) == 0 {
		panic("cannot register directive with empty name")
	}
	for _, char := range directive.Name {
		if char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char == '_' {
			continue
		}
		panic(string(char) + " in " + directive.Name + " is not allowed as directive name")
	}
	if directive.Where == nil {
		panic("Where must be defined")
	}
	if directive.Method == nil {
		panic("Method must be defined")
	}
	methodReflection := reflect.ValueOf(directive.Method)
	if methodReflection.IsNil() {
		panic("Method must be defined")
	}
}

var _ = RegisterDirective(Directive{
	Name: "skip",
	Where: []DirectiveLocation{
		DirectiveLocationField,
		DirectiveLocationFragment,
		DirectiveLocationFragmentInline,
	},
	Method: func(args struct{ If bool }) DirectiveModdifier {
		return DirectiveModdifier{
			Skip: args.If,
		}
	},
	Description: "Directs the executor to skip this field or fragment when the `if` argument is true.",
})

var _ = RegisterDirective(Directive{
	Name: "include",
	Where: []DirectiveLocation{
		DirectiveLocationField,
		DirectiveLocationFragment,
		DirectiveLocationFragmentInline,
	},
	Method: func(args struct{ If bool }) DirectiveModdifier {
		return DirectiveModdifier{
			Skip: !args.If,
		}
	},
	Description: "Directs the executor to include this field or fragment only when the `if` argument is true.",
})

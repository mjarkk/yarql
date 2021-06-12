package graphql

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"unicode"
)

type operator struct {
	operationType       string // "query" || "mutation" || "subscription" || "fragment"
	name                string // "" = no name given, note: fragments always have a name
	selection           selectionSet
	directives          directives
	variableDefinitions variableDefinitions
	fragment            *inlineFragment // defined if: operationType == "fragment"
}

type selectionSet []selection

type selection struct {
	selectionType  string          // "Field" || "FragmentSpread" || "InlineFragment"
	field          *field          // Optional
	fragmentSpread *fragmentSpread // Optional
	inlineFragment *inlineFragment // Optional
}

type field struct {
	name       string
	alias      string       // Optional
	selection  selectionSet // Optional
	directives directives   // Optional
	arguments  arguments    // Optional
}

type fragmentSpread struct {
	name       string
	directives directives // Optional
}

type inlineFragment struct {
	selection           selectionSet
	onTypeConditionName string     // Optional
	directives          directives // Optional
}

type directives map[string]directive

type directive struct {
	name      string
	arguments arguments
}

type typeReference struct {
	list    bool
	nonNull bool

	// list == false
	name string

	// list == true
	listType *typeReference
}

type variableDefinitions map[string]variableDefinition // Key is the variable name without the $

type variableDefinition struct {
	name         string
	varType      typeReference
	defaultValue *value
}

type arguments map[string]value

func parseQuery(input string) ([]*operator, *ErrorWLocation) {
	res := []*operator{}
	iter := &iter{
		data: input,
	}

	for {
		operator, err := iter.parseOperatorOrFragment()
		if err != nil {
			return nil, err
		}
		if operator == nil {
			return res, err
		}
		res = append(res, operator)
	}
}

type iter struct {
	data   string
	charNr uint64
}

type ErrorWLocation struct {
	err    error
	line   uint
	column uint
}

func (e ErrorWLocation) Error() string {
	return e.err.Error()
}

func (i *iter) err(err string) *ErrorWLocation {
	line := uint(1)
	column := uint(0)
	for idx, char := range i.data {
		if uint64(idx) == i.charNr {
			break
		}

		switch char {
		case '\n':
			if column == 0 && idx > 0 && i.data[idx-1] == '\r' {
				// don't count \r\n as 2 lines
				continue
			}
			line++
			column = 0
		case '\r':
			line++
			column = 0
		default:
			column++
		}
	}

	return &ErrorWLocation{
		errors.New(err),
		line,
		uint(column),
	}
}

func (i *iter) unexpectedEOF() *ErrorWLocation {
	return i.err("unexpected EOF")
}

func (i *iter) checkC(nr uint64) (res rune, end bool) {
	if i.eof(nr) {
		return 0, true
	}
	return i.c(nr), false
}

func (i *iter) c(nr uint64) rune {
	return rune(i.data[nr])
}

func (i *iter) eof(nr uint64) bool {
	return nr >= uint64(len(i.data))
}

func (i *iter) currentC() rune {
	return i.c(i.charNr)
}

// Parses one of the following:
// - https://spec.graphql.org/June2018/#sec-Language.Operations
// - https://spec.graphql.org/June2018/#FragmentDefinition
func (i *iter) parseOperatorOrFragment() (*operator, *ErrorWLocation) {
	res := operator{
		operationType:       "query",
		name:                "",
		selection:           selectionSet{},
		directives:          directives{},
		variableDefinitions: variableDefinitions{},
	}

	// This can only return EOF errors atm and as we handle those differently here we can ignore the error
	c, _ := i.mightIgnoreNextTokens()
	if i.eof(i.charNr) {
		return nil, nil
	}

	var err *ErrorWLocation

	// For making a simple query you don't have to define a operation type
	// Note that a simple query as descried above disables the name, variable definitions and directives
	if c != '{' {
		newOperationType := i.matches("query", "mutation", "subscription", "fragment")
		if len(newOperationType) == 0 {
			return nil, i.err("unknown operation type")
		}
		res.operationType = newOperationType

		c, err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if c != '(' && c != '@' && c != '{' || res.operationType == "fragment" {
			name, err := i.parseName()
			if err != nil {
				return nil, err
			}
			if name == "" {
				return nil, i.err("expected name but got \"" + string(i.currentC()) + "\"")
			}
			res.name = name

			c, err = i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}
		}

		if res.operationType == "fragment" {
			if i.matches("on") == "" {
				return nil, i.err("expected type condition (\"on some_name\")")
			}
			res.fragment, err = i.parseInlineFragment(true)
			if err != nil {
				return nil, err
			}
			return &res, nil
		}

		if c == '(' {
			i.charNr++
			variableDefinitions, err := i.parseVariableDefinitions()
			if err != nil {
				return nil, err
			}
			res.variableDefinitions = variableDefinitions
			c, err = i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}
		} else if c != '@' && c != '{' {
			return nil, i.err("unexpected character")
		}

		if c == '@' {
			directives, err := i.parseDirectives()
			if err != nil {
				return nil, err
			}
			res.directives = directives
		} else if c != '{' {
			return nil, i.err("unexpected character")
		}
	}

	i.charNr++
	selection, err := i.parseSelectionSets()
	if err != nil {
		return nil, err
	}
	res.selection = selection
	return &res, nil
}

// https://spec.graphql.org/June2018/#VariableDefinitions
func (i *iter) parseVariableDefinitions() (variableDefinitions, *ErrorWLocation) {
	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}

	res := variableDefinitions{}
	for {
		if c == ')' {
			i.charNr++
			return res, nil
		}

		variable, err := i.parseVariableDefinition()
		if err != nil {
			return nil, err
		}

		c, err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
		if c == ',' {
			i.charNr++
			c, err = i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}
		}

		res[variable.name] = variable
	}
}

// https://spec.graphql.org/June2018/#VariableDefinition
func (i *iter) parseVariableDefinition() (variableDefinition, *ErrorWLocation) {
	res := variableDefinition{}

	// Parse var name
	varName, err := i.parseVariable(false)
	if err != nil {
		return res, err
	}
	res.name = varName

	// Parse identifier for switching from var name to var type
	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return res, err
	}
	if c != ':' {
		return res, i.err(`expected ":" but got "` + string(i.currentC()) + `"`)
	}
	i.charNr++

	// Parse variable type
	_, err = i.mightIgnoreNextTokens()
	if err != nil {
		return res, err
	}
	varType, err := i.parseType()
	if err != nil {
		return res, err
	}
	res.varType = *varType

	// Parse optional default value
	c, err = i.mightIgnoreNextTokens()
	if err != nil {
		return res, err
	}
	if c == '=' {
		i.charNr++
		_, err = i.mightIgnoreNextTokens()
		if err != nil {
			return res, err
		}

		value, err := i.parseValue()
		if err != nil {
			return res, err
		}
		res.defaultValue = &value

		_, err = i.mightIgnoreNextTokens()
		if err != nil {
			return res, err
		}
	}

	return res, nil
}

// https://spec.graphql.org/June2018/#Value
func (i *iter) parseValue() (value, *ErrorWLocation) {
	switch i.currentC() {
	case '$':
		i.charNr++
		varName, err := i.parseVariable(true)
		return makeVariableValue(varName), err
	case '-', '1', '2', '3', '4', '5', '6', '7', '8', '9', '0':
		return i.parseNumberValue()
	case '"':
		val, err := i.parseString()
		return makeStringValue(val), err
	case '[':
		i.charNr++
		list, err := i.parseListValue()
		return makeArrayValue(list), err
	case '{':
		i.charNr++
		values, err := i.parseArgumentsOrObjectValues('}')
		return makeStructValue(values), err
	default:
		name, err := i.parseName()
		if err != nil {
			return value{}, err
		}
		switch name {
		case "null":
			return makeNullValue(), nil
		case "true", "false":
			return makeBooleanValue(name == "true"), nil
		case "":
			return value{}, i.err("invalid value")
		default:
			return makeEnumValue(name), nil
		}
	}
}

func (i *iter) parseString() (string, *ErrorWLocation) {
	res := []byte{}
	isBlock := false
	if i.matches(`"""`) == `"""` {
		isBlock = true
	} else {
		// is normal string
		i.charNr++
	}

	for {
		c, eof := i.checkC(i.charNr)
		if eof {
			return "", i.unexpectedEOF()
		}
		i.charNr++
		switch c {
		case '"':
			if !isBlock {
				return string(res), nil
			}

			c2, eof := i.checkC(i.charNr)
			if eof {
				return "", i.unexpectedEOF()
			}

			if c2 == '"' {
				i.charNr++
				c3, eof := i.checkC(i.charNr)
				if eof {
					return "", i.unexpectedEOF()
				}

				if c3 == '"' {
					i.charNr++
					// TODO: this trim space is wrong, only the leading and trailing empty lines should be removed not all the spaces chars before the first char
					//       for more info see: https://spec.graphql.org/June2018/#BlockStringCharacter
					return string(bytes.TrimSpace(res)), nil
				}
				res = append(res, '"')
			}
			res = append(res, '"')
		case '\r', '\n':
			if !isBlock {
				return "", i.err("carriage return and new lines not allowed in a string, to use these characters use a block string")
			}
			res = append(res, byte(c))
		case '\\':
			// next char is escaped
			// Note: in a blockstring this char probably wrong diffrent

			c, eof = i.checkC(i.charNr)
			if eof {
				return "", i.unexpectedEOF()
			}
			i.charNr++
			switch c {
			case 'u':
				unicodeChars := []byte{}

				// https://spec.graphql.org/June2018/#EscapedUnicode
				for {
					c, eof = i.checkC(i.charNr)
					if eof {
						return "", i.unexpectedEOF()
					}

					if !unicode.Is(unicode.Hex_Digit, c) {
						res = append(res, 'u')
						res = append(res, unicodeChars...)
						break
					}

					i.charNr++
					unicodeChars = append(unicodeChars, byte(c))
					if len(unicodeChars) == 4 {
						from := 0
						if unicodeChars[0] == '0' && unicodeChars[1] == '0' {
							from = 2
							if unicodeChars[2] == '0' && unicodeChars[3] == '0' {
								from = 4
							}
						}

						if from == 4 {
							break
						}

						dst := make([]byte, hex.DecodedLen(len(unicodeChars[from:])))
						n, err := hex.Decode(dst, unicodeChars[from:])
						if err != nil {
							res = append(res, 'u')
							res = append(res, unicodeChars...)
						} else if n == 1 {
							res = append(res, dst[0])
						} else {
							char := rune(dst[0])<<8 | rune(dst[1])
							res = append(res, []byte(string(char))...)
						}
						break
					}
				}

			case 'b':
				res = append(res, byte('\b'))
			case 'f':
				res = append(res, byte('\f'))
			case 'n':
				res = append(res, byte('\n'))
			case 'r':
				res = append(res, byte('\r'))
			case 't':
				res = append(res, byte('\t'))
			default:
				res = append(res, byte(c))
			}
		default:
			res = append(res, byte(c))
		}
	}
}

// https://spec.graphql.org/June2018/#ListValue
func (i *iter) parseListValue() ([]value, *ErrorWLocation) {
	res := []value{}

	firstLoop := true
	for {
		c, err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
		if c == ']' {
			i.charNr++
			return res, nil
		}

		if !firstLoop && c == ',' {
			i.charNr++
			c, err := i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}

			if c == ']' {
				i.charNr++
				return res, nil
			}
		}

		val, err := i.parseValue()
		if err != nil {
			return nil, err
		}
		res = append(res, val)
		firstLoop = false
	}
}

// Returns FloatValue or IntValue
// https://spec.graphql.org/June2018/#FloatValue
// https://spec.graphql.org/June2018/#IntValue
func (i *iter) parseNumberValue() (value, *ErrorWLocation) {
	toMap := func(list string) map[rune]bool {
		res := map[rune]bool{}
		for _, char := range list {
			res[char] = true
		}
		return res
	}
	digit := toMap("0123456789")

	resStr := ""
	res := func(isFloat bool) (value, *ErrorWLocation) {
		if !isFloat {
			// Value is int
			intValue, err := strconv.Atoi(resStr)
			if err != nil {
				return value{}, i.err("unable to parse int")
			}
			return makeIntValue(intValue), nil
		}

		floatValue, err := strconv.ParseFloat(resStr, 64)
		if err != nil {
			return value{}, i.err("unable to parse float")
		}

		return makeFloatValue(floatValue), nil
	}

	c := i.currentC()
	if c == '-' {
		resStr += string(c)
		i.charNr++
	}

	// parse integer part
	c, eof := i.checkC(i.charNr)
	if eof {
		return value{}, i.unexpectedEOF()
	}
	if c == '0' {
		resStr += string(c)
		i.charNr++

		c, eof = i.checkC(i.charNr)
		if eof {
			return value{}, i.unexpectedEOF()
		}
		if c != '.' && c != 'e' && c != 'E' {
			return res(false)
		}
	} else if _, ok := digit[c]; ok {
		resStr += string(c)
		i.charNr++

		for {
			c, eof := i.checkC(i.charNr)
			if eof {
				return value{}, i.unexpectedEOF()
			}

			if c == '.' || c == 'e' || c == 'E' {
				break
			}

			_, ok := digit[c]
			if !ok {
				return res(false)
			}

			resStr += string(c)
			i.charNr++
		}
	} else {
		return value{}, i.err("not a valid int or float")
	}

	// Parse optional float fractional part
	c, eof = i.checkC(i.charNr)
	if eof {
		return value{}, i.unexpectedEOF()
	}
	if c == '.' {
		resStr += string(c)

		// Tread the first number of the fractional part diffrent as it is required
		i.charNr++
		c, eof = i.checkC(i.charNr)
		if eof {
			return value{}, i.unexpectedEOF()
		}

		_, ok := digit[c]
		if !ok {
			return value{}, i.err("not a valid float")
		}
		resStr += string(c)

		for {
			i.charNr++
			c, eof = i.checkC(i.charNr)
			if eof {
				return value{}, i.unexpectedEOF()
			}

			if c == 'e' || c == 'E' {
				break
			}

			_, ok := digit[c]
			if !ok {
				return res(true)
			}

			resStr += string(c)
		}
	}

	// Parse optional float exponent part
	c, eof = i.checkC(i.charNr)
	if eof {
		return value{}, i.unexpectedEOF()
	}
	if c != 'e' && c != 'E' {
		// We can assume here the value is a float as the this code can only be reached if the value contains "." or "e" or "E"
		return res(true)
	}
	resStr += string(c)

	i.charNr++
	c, eof = i.checkC(i.charNr)
	if eof {
		return value{}, i.unexpectedEOF()
	}
	if c == '+' || c == '-' {
		resStr += string(c)

		i.charNr++
		c, eof = i.checkC(i.charNr)
		if eof {
			return value{}, i.unexpectedEOF()
		}
	}

	_, ok := digit[c]
	if !ok {
		return value{}, i.err("not a valid float")
	}
	resStr += string(c)

	for {
		i.charNr++
		c, eof = i.checkC(i.charNr)
		if eof {
			return value{}, i.unexpectedEOF()
		}

		_, ok := digit[c]
		if !ok {
			return res(true)
		}
		resStr += string(c)
	}
}

// https://spec.graphql.org/June2018/#Type
func (i *iter) parseType() (*typeReference, *ErrorWLocation) {
	res := typeReference{}

	if i.currentC() == '[' {
		res.list = true
		i.charNr++
		_, err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		res.listType, err = i.parseType()
		if err != nil {
			return nil, err
		}

		c, err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
		if c != ']' {
			return nil, i.err(`expected list closure ("]") but got "` + string(c) + `"`)
		}
		i.charNr++
	} else {
		name, err := i.parseName()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, i.err("type name missing or invalid type name")
		}
		res.name = name
	}

	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c == '!' {
		res.nonNull = true
		i.charNr++
	}

	return &res, nil
}

// https://spec.graphql.org/June2018/#Variable
func (i *iter) parseVariable(alreadyParsedIdentifier bool) (string, *ErrorWLocation) {
	if !alreadyParsedIdentifier {
		i.mightIgnoreNextTokens()
		if i.currentC() != '$' {
			return "", i.err(`variable must start with "$"`)
		}
		i.charNr++
	}

	name, err := i.parseName()
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", i.err("cannot have empty variable name")
	}
	if name == "null" {
		return "", i.err("null is a illegal variable name")
	}

	return name, nil
}

// https://spec.graphql.org/June2018/#sec-Selection-Sets
func (i *iter) parseSelectionSets() (selectionSet, *ErrorWLocation) {
	res := selectionSet{}

	for {
		c, err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if c == '}' {
			i.charNr++
			return res, nil
		}

		selection, err := i.parseSelection()
		if err != nil {
			return nil, err
		}
		res = append(res, selection)

		c, err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		switch c {
		case ',':
			i.charNr++
		case '}':
			i.charNr++
			return res, nil
		}
	}
}

// https://spec.graphql.org/June2018/#Selection
func (i *iter) parseSelection() (selection, *ErrorWLocation) {
	res := selection{}

	if len(i.matches("...")) > 0 {
		_, err := i.mightIgnoreNextTokens()
		if err != nil {
			return res, err
		}

		name, err := i.parseName()
		if err != nil {
			return res, err
		}

		if name == "on" || name == "" {
			inlineFragment, err := i.parseInlineFragment(name == "on")
			if err != nil {
				return res, err
			}
			res.selectionType = "InlineFragment"
			res.inlineFragment = inlineFragment
		} else {
			fragmentSpread, err := i.parseFragmentSpread(name)
			if err != nil {
				return res, err
			}
			res.selectionType = "FragmentSpread"
			res.fragmentSpread = fragmentSpread
		}
	} else {
		field, err := i.parseField()
		if err != nil {
			return res, err
		}
		res.selectionType = "Field"
		res.field = field
	}

	return res, nil
}

// https://spec.graphql.org/June2018/#InlineFragment
func (i *iter) parseInlineFragment(hasTypeCondition bool) (*inlineFragment, *ErrorWLocation) {
	res := inlineFragment{}
	if hasTypeCondition {
		_, err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		res.onTypeConditionName, err = i.parseName()
		if err != nil {
			return nil, err
		}

		if res.onTypeConditionName == "" {
			return nil, i.err("cannot have type condition without name")
		}
	}

	// parse optional directives
	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c == '@' {
		res.directives, err = i.parseDirectives()
		if err != nil {
			return nil, err
		}
	}

	// Parse SelectionSet
	c, err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c != '{' {
		return nil, i.err("expected \"{\", not: \"" + string(i.currentC()) + "\"")
	}
	i.charNr++
	res.selection, err = i.parseSelectionSets()
	return &res, err
}

// https://spec.graphql.org/June2018/#FragmentSpread
func (i *iter) parseFragmentSpread(name string) (*fragmentSpread, *ErrorWLocation) {
	res := fragmentSpread{name: name}

	// parse optional directives
	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c == '@' {
		res.directives, err = i.parseDirectives()
		if err != nil {
			return nil, err
		}
	}

	return &res, nil
}

// https://spec.graphql.org/June2018/#Field
func (i *iter) parseField() (*field, *ErrorWLocation) {
	res := field{}

	// Parse name (and alias if pressent)
	nameOrAlias, err := i.parseName()
	if err != nil {
		return nil, err
	}
	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}

	if c == ':' {
		if nameOrAlias == "" {
			return nil, i.err("field alias should have a name")
		}
		res.alias = nameOrAlias
		i.charNr++
		_, err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
		res.name, err = i.parseName()
		if err != nil {
			return nil, err
		}
		if res.name == "" {
			return nil, i.err("field should have a name")
		}
	} else {
		if nameOrAlias == "" {
			return nil, i.err("field should have a name")
		}
		res.name = nameOrAlias
	}

	// Parse Arguments if present
	c, err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c == '(' {
		i.charNr++
		args, err := i.parseArgumentsOrObjectValues(')')
		if err != nil {
			return nil, err
		}
		res.arguments = args
	}

	// Parse directives if present
	c, err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c == '@' {
		directives, err := i.parseDirectives()
		if err != nil {
			return nil, err
		}
		res.directives = directives
	}

	// Parse SelectionSet if pressent
	c, err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c == '{' {
		i.charNr++
		selection, err := i.parseSelectionSets()
		if err != nil {
			return nil, err
		}
		res.selection = selection
	}

	return &res, nil
}

// Parses object values and arguments as the only diffrents seems to be the wrappers around it
// ObjectValues > https://spec.graphql.org/June2018/#ObjectValue
// Arguments > https://spec.graphql.org/June2018/#Arguments
func (i *iter) parseArgumentsOrObjectValues(closure rune) (arguments, *ErrorWLocation) {
	res := arguments{}

	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}

	if c == closure {
		i.charNr++
		return res, nil
	}

	for {
		name, err := i.parseName()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, i.err("argument name must be defined")
		}

		c, err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if c != ':' {
			return nil, i.err("expected \":\"")
		}
		i.charNr++

		_, err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		value, err := i.parseValue()
		if err != nil {
			return nil, err
		}
		res[name] = value

		c, err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if c == ',' {
			i.charNr++

			c, err = i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}
		}

		if c == closure {
			i.charNr++
			return res, nil
		}
	}
}

// https://spec.graphql.org/June2018/#Directives
func (i *iter) parseDirectives() (directives, *ErrorWLocation) {
	res := directives{}
	for {
		c, err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if c != '@' {
			return res, nil
		}

		i.charNr++
		directive, err := i.parseDirective()
		if err != nil {
			return nil, err
		}
		res[directive.name] = *directive
	}
}

// https://spec.graphql.org/June2018/#Directive
func (i *iter) parseDirective() (*directive, *ErrorWLocation) {
	name, err := i.parseName()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, i.err("directive must have a name")
	}
	res := directive{name: name}

	// Parse optional Arguments
	c, err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if c == '(' {
		i.charNr++
		args, err := i.parseArgumentsOrObjectValues(')')
		if err != nil {
			return nil, err
		}
		res.arguments = args
	}

	return &res, nil
}

// https://spec.graphql.org/June2018/#Name
func (i *iter) parseName() (string, *ErrorWLocation) {
	allowedChars := map[rune]bool{}

	letters := "abcdefghijklmnopqrstuvwxyz"
	numbers := "0123456789"
	special := "_"
	for _, allowedChar := range []byte(letters + strings.ToUpper(letters) + special) {
		allowedChars[rune(allowedChar)] = true
		allowedChars[rune(allowedChar)] = true
	}
	for _, notFirstAllowedChar := range []byte(numbers) {
		allowedChars[rune(notFirstAllowedChar)] = false
	}

	name := ""
	for {
		c, eof := i.checkC(i.charNr)
		if eof {
			return name, i.unexpectedEOF()
		}

		allowedAsFirstChar, ok := allowedChars[c]
		if !ok {
			return name, nil
		}

		if name == "" && !allowedAsFirstChar {
			return name, nil
		}

		name += string(c)

		i.charNr++
	}
}

// https://spec.graphql.org/June2018/#sec-Source-Text.Ignored-Tokens
func (i *iter) isIgnoredToken(c rune) bool {
	return isUnicodeBom(c) || isWhiteSpace(c) || i.isLineTerminator() || i.isComment(true)
}

func (i *iter) mightIgnoreNextTokens() (rune, *ErrorWLocation) {
	for {
		c, eof := i.checkC(i.charNr)
		if eof {
			return 0, i.unexpectedEOF()
		}

		isIgnoredChar := i.isIgnoredToken(c)
		if !isIgnoredChar {
			return c, nil
		}

		i.charNr++
	}
}

// https://spec.graphql.org/June2018/#UnicodeBOM
func isUnicodeBom(input rune) bool {
	return input == '\uFEFF'
}

// https://spec.graphql.org/June2018/#WhiteSpace
func isWhiteSpace(input rune) bool {
	return input == ' ' || input == '\t'
}

// https://spec.graphql.org/June2018/#LineTerminator
func (i *iter) isLineTerminator() bool {
	c := i.currentC()
	if c == '\n' {
		return true
	}
	if c == '\r' {
		next, _ := i.checkC(i.charNr + 1)
		if next == '\n' {
			i.charNr++
		}
		return true
	}
	return false
}

// https://spec.graphql.org/June2018/#Comment
func (i *iter) isComment(parseComment bool) bool {
	if i.currentC() == '#' {
		if parseComment {
			i.parseComment()
		}
		return true
	}
	return false
}

func (i *iter) parseComment() {
	for {
		if i.eof(i.charNr) {
			return
		}
		if i.isLineTerminator() {
			return
		}
		i.charNr++
	}
}

func (i *iter) matches(oneOf ...string) string {
	startIdx := i.charNr

	oneOfMap := map[string]bool{}
	for _, val := range oneOf {
		oneOfMap[val] = true
	}

	for {
		c, eof := i.checkC(i.charNr)
		if eof {
			i.charNr = startIdx
			return ""
		}
		offset := i.charNr - startIdx

		for key := range oneOfMap {
			keyLen := uint64(len(key))
			if offset >= keyLen || rune(key[offset]) != c {
				delete(oneOfMap, key)
			} else if keyLen == offset+1 {
				i.charNr++
				return key
			}
		}

		if len(oneOfMap) == 0 {
			i.charNr = startIdx
			return ""
		}

		i.charNr++
	}
}

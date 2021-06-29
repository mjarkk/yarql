package graphql

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strconv"
	"unicode"
)

type operator struct {
	operationType       string // "query" || "mutation" || "subscription" || "fragment"
	name                string // "" = no name given, note: fragments always have a name
	selectionIdx        int
	directives          directives
	variableDefinitions variableDefinitions
	fragment            inlineFragment // defined if: operationType == "fragment"
}

type selectionSet []selection

type selection struct {
	selectionType  string         // "Field" || "FragmentSpread" || "InlineFragment" // TODO change this to an enum
	field          field          // Optional
	fragmentSpread fragmentSpread // Optional
	inlineFragment inlineFragment // Optional
}

type field struct {
	name         string
	alias        []byte     // Optional
	selectionIdx int        // Optional
	directives   directives // Optional
	arguments    arguments  // Optional
}

type fragmentSpread struct {
	name       string
	directives directives // Optional
}

type inlineFragment struct {
	selectionIdx        int
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

type iterT struct {
	data                 string
	charNr               uint64
	unknownQueries       int
	unknownMutations     int
	unknownSubscriptions int
	fragments            map[string]operator
	operatorsMap         map[string]operator
	resErrors            []ErrorWLocation
	selections           []selectionSet
	nameBuff             []byte
	stringBuff           []byte
	selectionSetIdx      int
}

type ErrorWLocation struct {
	err    error
	line   uint
	column uint
}

func (e ErrorWLocation) Error() string {
	return e.err.Error()
}

func (i *iterT) parseQuery(input string) {
	*i = iterT{
		data:         input,
		resErrors:    i.resErrors[:0],
		fragments:    map[string]operator{},
		operatorsMap: map[string]operator{},
		selections:   i.selections,
		nameBuff:     i.nameBuff[:0],
		stringBuff:   i.stringBuff[:0],
	}

	for {
		criticalErr := i.parseOperatorOrFragment()
		if criticalErr || i.eof(i.charNr) {
			return
		}
	}
}

func (i *iterT) err(err string) bool {
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

	i.resErrors = append(i.resErrors, ErrorWLocation{
		errors.New(err),
		line,
		uint(column),
	})
	return true
}

func (i *iterT) unexpectedEOF() bool {
	return i.err("unexpected EOF")
}

func (i *iterT) checkC(nr uint64) (res byte, end bool) {
	if i.eof(nr) {
		return 0, true
	}
	return i.c(nr), false
}

func (i *iterT) c(nr uint64) byte {
	return i.data[nr]
}

func (i *iterT) eof(nr uint64) bool {
	return nr >= uint64(len(i.data))
}

func (i *iterT) currentC() byte {
	return i.c(i.charNr)
}

// Parses one of the following:
// - https://spec.graphql.org/June2018/#sec-Language.Operations
// - https://spec.graphql.org/June2018/#FragmentDefinition
func (i *iterT) parseOperatorOrFragment() bool {
	res := operator{
		operationType:       "query",
		directives:          directives{},
		variableDefinitions: variableDefinitions{},
		selectionIdx:        -1,
	}

	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return false
	}

	// For making a simple query you don't have to define a operation type
	// Note that a simple query as descried above disables the name, variable definitions and directives
	if c != '{' {
		newOperationType := i.matches("query", "mutation", "subscription", "fragment")
		if len(newOperationType) == 0 {
			return i.err("unknown operation type")
		}
		res.operationType = newOperationType

		var eof bool
		c, eof = i.mightIgnoreNextTokens()
		if eof {
			return i.unexpectedEOF()
		}

		if c != '(' && c != '@' && c != '{' || res.operationType == "fragment" {
			var criticalErr bool
			i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
			if criticalErr {
				return criticalErr
			}
			if len(i.nameBuff) == 0 {
				return i.err("expected name but got \"" + string(i.currentC()) + "\"")
			}
			res.name = string(i.nameBuff)

			c, eof = i.mightIgnoreNextTokens()
			if eof {
				return i.unexpectedEOF()
			}
		}

		if res.operationType == "fragment" {
			if i.matches("on") == "" {
				return i.err("expected type condition (\"on some_name\")")
			}

			var criticalErr bool
			res.fragment, criticalErr = i.parseInlineFragment(true)
			if criticalErr {
				return criticalErr
			}

			if res.name == "" {
				i.err("fragment cannot have an empty name")
				return false // the above is not a critical parsing error
			}
			if _, ok := i.fragments[res.name]; ok {
				i.err("fragment name can only be used once (name = \"" + res.name + "\")")
				return false // the above is not a critical parsing error
			}
			i.fragments[res.name] = res
			return false
		}

		if c == '(' {
			i.charNr++
			variableDefinitions, criticalErr := i.parseVariableDefinitions()
			if criticalErr {
				return criticalErr
			}
			res.variableDefinitions = variableDefinitions
			c, eof = i.mightIgnoreNextTokens()
			if eof {
				return i.unexpectedEOF()
			}
		} else if c != '@' && c != '{' {
			return i.err("unexpected character")
		}

		if c == '@' {
			directives, criticalErr := i.parseDirectives()
			if criticalErr {
				return criticalErr
			}
			res.directives = directives
		} else if c != '{' {
			return i.err("unexpected character")
		}
	}

	i.charNr++
	var criticalErr bool
	res.selectionIdx, criticalErr = i.parseSelectionSets()

	if criticalErr {
		return criticalErr
	}

	if res.name == "" {
		switch res.operationType {
		case "query":
			i.unknownQueries++
			res.name = "unknown_query_" + strconv.Itoa(i.unknownQueries)
		case "mutation":
			i.unknownMutations++
			res.name = "unknown_mutation_" + strconv.Itoa(i.unknownMutations)
		case "subscription":
			i.unknownSubscriptions++
			res.name = "unknown_subscription_" + strconv.Itoa(i.unknownSubscriptions)
		}
	}
	if _, ok := i.operatorsMap[res.name]; ok {
		i.err("operator name can only be used once (name = \"" + res.name + "\")")
		return false // the above is not a critical parsing error
	}

	i.operatorsMap[res.name] = res
	return false
}

// https://spec.graphql.org/June2018/#VariableDefinitions
func (i *iterT) parseVariableDefinitions() (variableDefinitions, bool) {
	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return nil, i.unexpectedEOF()
	}

	res := variableDefinitions{}
	for {
		if c == ')' {
			i.charNr++
			return res, false
		}

		variable, criticalErr := i.parseVariableDefinition()
		if criticalErr {
			return nil, criticalErr
		}

		c, eof = i.mightIgnoreNextTokens()
		if eof {
			return nil, i.unexpectedEOF()
		}
		if c == ',' {
			i.charNr++
			c, eof = i.mightIgnoreNextTokens()
			if eof {
				return nil, i.unexpectedEOF()
			}
		}

		res[variable.name] = variable
	}
}

// https://spec.graphql.org/June2018/#VariableDefinition
func (i *iterT) parseVariableDefinition() (variableDefinition, bool) {
	res := variableDefinition{}

	// Parse var name
	varName, criticalErr := i.parseVariable(false)
	if criticalErr {
		return res, criticalErr
	}
	res.name = varName

	// Parse identifier for switching from var name to var type
	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c != ':' {
		return res, i.err(`expected ":" but got "` + string(i.currentC()) + `"`)
	}
	i.charNr++

	// Parse variable type
	_, eof = i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	varType, criticalErr := i.parseType()
	if criticalErr {
		return res, criticalErr
	}
	res.varType = *varType

	// Parse optional default value
	c, eof = i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c == '=' {
		i.charNr++
		_, eof = i.mightIgnoreNextTokens()
		if eof {
			return res, i.unexpectedEOF()
		}

		value, criticalErr := i.parseValue(false)
		if criticalErr {
			return res, criticalErr
		}
		res.defaultValue = &value

		_, eof = i.mightIgnoreNextTokens()
		if eof {
			return res, i.unexpectedEOF()
		}
	}

	return res, false
}

// https://spec.graphql.org/June2018/#Value
func (i *iterT) parseValue(allowVariables bool) (value, bool) {
	switch i.currentC() {
	case '$':
		if !allowVariables {
			return value{}, i.err("variables not allowed within this context")
		}
		i.charNr++
		varName, criticalErr := i.parseVariable(true)
		return makeVariableValue(varName), criticalErr
	case '-', '1', '2', '3', '4', '5', '6', '7', '8', '9', '0':
		return i.parseNumberValue()
	case '"':
		val, criticalErr := i.parseString(i.stringBuff[:0])
		return makeStringValue(val), criticalErr
	case '[':
		i.charNr++
		list, criticalErr := i.parseListValue(allowVariables)
		return makeArrayValue(list), criticalErr
	case '{':
		i.charNr++
		values, criticalErr := i.parseArgumentsOrObjectValues('}')
		return makeStructValue(values), criticalErr
	default:
		var criticalErr bool
		i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
		if criticalErr {
			return value{}, criticalErr
		}
		strName := string(i.nameBuff)
		switch strName {
		case "null":
			return makeNullValue(), false
		case "true":
			return makeBooleanValue(true), false
		case "false":
			return makeBooleanValue(false), false
		case "":
			return value{}, i.err("invalid value")
		default:
			return makeEnumValue(strName), false
		}
	}
}

func (i *iterT) parseString(res []byte) (string, bool) {
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
				return string(res), false
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
					return string(bytes.TrimSpace(res)), false
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

					if !unicode.Is(unicode.Hex_Digit, rune(c)) {
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
func (i *iterT) parseListValue(allowVariables bool) ([]value, bool) {
	res := []value{}

	firstLoop := true
	for {
		c, eof := i.mightIgnoreNextTokens()
		if eof {
			return nil, i.unexpectedEOF()
		}
		if c == ']' {
			i.charNr++
			return res, false
		}

		if !firstLoop && c == ',' {
			i.charNr++
			c, eof := i.mightIgnoreNextTokens()
			if eof {
				return nil, i.unexpectedEOF()
			}

			if c == ']' {
				i.charNr++
				return res, false
			}
		}

		val, criticalErr := i.parseValue(allowVariables)
		if criticalErr {
			return nil, criticalErr
		}
		res = append(res, val)
		firstLoop = false
	}
}

// Returns FloatValue or IntValue
// https://spec.graphql.org/June2018/#FloatValue
// https://spec.graphql.org/June2018/#IntValue
func (i *iterT) parseNumberValue() (value, bool) {
	digit := map[byte]bool{
		'0': true,
		'1': true,
		'2': true,
		'3': true,
		'4': true,
		'5': true,
		'6': true,
		'7': true,
		'8': true,
		'9': true,
	}

	resStr := ""
	res := func(isFloat bool) (value, bool) {
		if !isFloat {
			// Value is int
			intValue, err := strconv.Atoi(resStr)
			if err != nil {
				return value{}, i.err("unable to parse int")
			}
			return makeIntValue(intValue), false
		}

		floatValue, err := strconv.ParseFloat(resStr, 64)
		if err != nil {
			return value{}, i.err("unable to parse float")
		}

		return makeFloatValue(floatValue), false
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
func (i *iterT) parseType() (*typeReference, bool) {
	res := typeReference{}

	if i.currentC() == '[' {
		res.list = true
		i.charNr++
		_, eof := i.mightIgnoreNextTokens()
		if eof {
			return nil, i.unexpectedEOF()
		}

		var criticalErr bool
		res.listType, criticalErr = i.parseType()
		if criticalErr {
			return nil, criticalErr
		}

		c, eof := i.mightIgnoreNextTokens()
		if eof {
			return nil, i.unexpectedEOF()
		}
		if c != ']' {
			return nil, i.err(`expected list closure ("]") but got "` + string(c) + `"`)
		}
		i.charNr++
	} else {
		var criticalErr bool
		i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
		if criticalErr {
			return nil, criticalErr
		}
		if len(i.nameBuff) == 0 {
			return nil, i.err("type name missing or invalid type name")
		}
		res.name = string(i.nameBuff)
	}

	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return nil, i.unexpectedEOF()
	}
	if c == '!' {
		res.nonNull = true
		i.charNr++
	}

	return &res, false
}

// https://spec.graphql.org/June2018/#Variable
func (i *iterT) parseVariable(alreadyParsedIdentifier bool) (string, bool) {
	if !alreadyParsedIdentifier {
		_, eof := i.mightIgnoreNextTokens()
		if eof {
			return "", i.unexpectedEOF()
		}
		if i.currentC() != '$' {
			return "", i.err(`variable must start with "$"`)
		}
		i.charNr++
	}

	var criticalErr bool
	i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
	if criticalErr {
		return "", criticalErr
	}
	if len(i.nameBuff) == 0 {
		return "", i.err("cannot have empty variable name")
	}
	strName := string(i.nameBuff)
	if strName == "null" {
		return "", i.err("null is a illegal variable name")
	}

	return strName, false
}

// https://spec.graphql.org/June2018/#sec-Selection-Sets
func (i *iterT) parseSelectionSets() (int, bool) {
	setIdx := i.selectionSetIdx
	if setIdx >= len(i.selections) {
		i.selections = append(i.selections, []selection{})
	} else {
		i.selections[setIdx] = i.selections[setIdx][:0]
	}
	i.selectionSetIdx++

	for {
		c, eof := i.mightIgnoreNextTokens()
		if eof {
			return setIdx, i.unexpectedEOF()
		}

		if c == '}' {
			i.charNr++
			return setIdx, false
		}

		var criticalErr bool
		i.selections[setIdx], criticalErr = i.parseSelection(i.selections[setIdx])
		if criticalErr {
			return setIdx, criticalErr
		}

		c, eof = i.mightIgnoreNextTokens()
		if eof {
			return setIdx, i.unexpectedEOF()
		}

		switch c {
		case ',':
			i.charNr++
		case '}':
			i.charNr++
			return setIdx, false
		}
	}
}

// https://spec.graphql.org/June2018/#Selection
func (i *iterT) parseSelection(res selectionSet) (selectionSet, bool) {
	if len(i.matches("...")) > 0 {
		_, eof := i.mightIgnoreNextTokens()
		if eof {
			return res, i.unexpectedEOF()
		}

		var criticalErr bool
		i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
		if criticalErr {
			return res, criticalErr
		}
		name := string(i.nameBuff)

		if name == "on" || name == "" {
			inlineFragment, criticalErr := i.parseInlineFragment(name == "on")
			if criticalErr {
				return res, criticalErr
			}
			res = append(res, selection{
				selectionType:  "InlineFragment",
				inlineFragment: inlineFragment,
			})
		} else {
			fragmentSpread, criticalErr := i.parseFragmentSpread(name)
			if criticalErr {
				return res, criticalErr
			}
			res = append(res, selection{
				selectionType:  "FragmentSpread",
				fragmentSpread: fragmentSpread,
			})
		}
	} else {
		field, criticalErr := i.parseField()
		if criticalErr {
			return res, criticalErr
		}
		res = append(res, selection{
			selectionType: "Field",
			field:         field,
		})
	}

	return res, false
}

// https://spec.graphql.org/June2018/#InlineFragment
func (i *iterT) parseInlineFragment(hasTypeCondition bool) (inlineFragment, bool) {
	res := inlineFragment{
		selectionIdx: -1,
	}
	if hasTypeCondition {
		_, eof := i.mightIgnoreNextTokens()
		if eof {
			return res, i.unexpectedEOF()
		}

		var criticalErr bool
		i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
		res.onTypeConditionName = string(i.nameBuff)
		if criticalErr {
			return res, criticalErr
		}

		if res.onTypeConditionName == "" {
			return res, i.err("cannot have type condition without name")
		}
	}

	// parse optional directives
	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c == '@' {
		res.directives, eof = i.parseDirectives()
		if eof {
			return res, i.unexpectedEOF()
		}
	}

	// Parse SelectionSet
	c, eof = i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c != '{' {
		return res, i.err("expected \"{\", not: \"" + string(i.currentC()) + "\"")
	}
	i.charNr++
	var criticalErr bool
	res.selectionIdx, criticalErr = i.parseSelectionSets()
	return res, criticalErr
}

// https://spec.graphql.org/June2018/#FragmentSpread
func (i *iterT) parseFragmentSpread(name string) (fragmentSpread, bool) {
	res := fragmentSpread{name: name}

	// parse optional directives
	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c == '@' {
		res.directives, eof = i.parseDirectives()
		if eof {
			return res, i.unexpectedEOF()
		}
	}

	return res, false
}

// https://spec.graphql.org/June2018/#Field
func (i *iterT) parseField() (field, bool) {
	res := field{selectionIdx: -1}

	// Parse name (and alias if pressent)
	var criticalErr bool
	i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
	if criticalErr {
		return res, criticalErr
	}

	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}

	if c == ':' {
		if len(i.nameBuff) == 0 {
			return res, i.err("field alias should have a name")
		}
		res.alias = make([]byte, len(i.nameBuff))
		copy(res.alias, i.nameBuff)

		i.charNr++
		_, eof := i.mightIgnoreNextTokens()
		if eof {
			return res, i.unexpectedEOF()
		}
		i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
		if criticalErr {
			return res, criticalErr
		}
		res.name = string(i.nameBuff)
		if res.name == "" {
			return res, i.err("field should have a name")
		}
	} else {
		if len(i.nameBuff) == 0 {
			return res, i.err("field should have a name")
		}
		res.name = string(i.nameBuff)
	}

	// Parse Arguments if present
	c, eof = i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c == '(' {
		i.charNr++
		args, criticalErr := i.parseArgumentsOrObjectValues(')')
		if criticalErr {
			return res, criticalErr
		}
		res.arguments = args
	}

	// Parse directives if present
	c, eof = i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c == '@' {
		directives, criticalErr := i.parseDirectives()
		if criticalErr {
			return res, criticalErr
		}
		res.directives = directives
	}

	// Parse SelectionSet if pressent
	c, eof = i.mightIgnoreNextTokens()
	if eof {
		return res, i.unexpectedEOF()
	}
	if c == '{' {
		i.charNr++
		res.selectionIdx, criticalErr = i.parseSelectionSets()
		if criticalErr {
			return res, criticalErr
		}
	}

	return res, false
}

// Parses object values and arguments as the only diffrents seems to be the wrappers around it
// ObjectValues > https://spec.graphql.org/June2018/#ObjectValue
// Arguments > https://spec.graphql.org/June2018/#Arguments
func (i *iterT) parseArgumentsOrObjectValues(closure byte) (res arguments, criticalErr bool) {
	// FIXME this is slow

	res = arguments{}

	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return nil, i.unexpectedEOF()
	}

	if c == closure {
		i.charNr++
		return res, false
	}

	for {
		i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
		if criticalErr {
			return nil, criticalErr
		}
		if len(i.nameBuff) == 0 {
			return nil, i.err("argument name must be defined")
		}
		name := string(i.nameBuff)

		c, eof = i.mightIgnoreNextTokens()
		if eof {
			return nil, i.unexpectedEOF()
		}

		if c != ':' {
			return nil, i.err("expected \":\"")
		}
		i.charNr++

		_, eof = i.mightIgnoreNextTokens()
		if eof {
			return nil, i.unexpectedEOF()
		}

		value, criticalErr := i.parseValue(true)
		if criticalErr {
			return nil, criticalErr
		}
		res[name] = value

		c, eof = i.mightIgnoreNextTokens()
		if eof {
			return nil, eof
		}

		if c == ',' {
			i.charNr++

			c, eof = i.mightIgnoreNextTokens()
			if eof {
				return nil, i.unexpectedEOF()
			}
		}

		if c == closure {
			i.charNr++
			return res, false
		}
	}
}

// https://spec.graphql.org/June2018/#Directives
func (i *iterT) parseDirectives() (directives, bool) {
	res := directives{}
	for {
		c, eof := i.mightIgnoreNextTokens()
		if eof {
			return nil, i.unexpectedEOF()
		}

		if c != '@' {
			return res, false
		}

		i.charNr++
		directive, criticalErr := i.parseDirective()
		if criticalErr {
			return nil, criticalErr
		}
		res[directive.name] = *directive
	}
}

// https://spec.graphql.org/June2018/#Directive
func (i *iterT) parseDirective() (*directive, bool) {
	var criticalErr bool
	i.nameBuff, criticalErr = i.parseName(i.nameBuff[:0])
	if criticalErr {
		return nil, criticalErr
	}
	if len(i.nameBuff) == 0 {
		return nil, i.err("directive must have a name")
	}
	res := directive{name: string(i.nameBuff)}

	// Parse optional Arguments
	c, eof := i.mightIgnoreNextTokens()
	if eof {
		return nil, i.unexpectedEOF()
	}
	if c == '(' {
		i.charNr++
		args, criticalErr := i.parseArgumentsOrObjectValues(')')
		if criticalErr {
			return nil, criticalErr
		}
		res.arguments = args
	}

	return &res, false
}

// https://spec.graphql.org/June2018/#Name
func (i *iterT) parseName(name []byte) ([]byte, bool) {
	for {
		c, eof := i.checkC(i.charNr)
		if eof {
			return name, i.unexpectedEOF()
		}

		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (len(name) != 0 && c >= '0' && c <= '9') {
			name = append(name, c)
			i.charNr++
			continue
		}

		return name, false
	}
}

// https://spec.graphql.org/June2018/#sec-Source-Text.Ignored-Tokens
func (i *iterT) isIgnoredToken(c byte) bool {
	// TODO support unicode bomb
	return isWhiteSpace(c) || i.isLineTerminator() || i.isComment(true)
}

func (i *iterT) mightIgnoreNextTokens() (nextC byte, eof bool) {
	for {
		c, eof := i.checkC(i.charNr)
		if eof {
			return 0, true
		}

		isIgnoredChar := i.isIgnoredToken(c)
		if !isIgnoredChar {
			return c, false
		}

		i.charNr++
	}
}

// https://spec.graphql.org/June2018/#WhiteSpace
func isWhiteSpace(input byte) bool {
	return input == ' ' || input == '\t'
}

// https://spec.graphql.org/June2018/#LineTerminator
func (i *iterT) isLineTerminator() bool {
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
func (i *iterT) isComment(parseComment bool) bool {
	if i.currentC() == '#' {
		if parseComment {
			i.parseComment()
		}
		return true
	}
	return false
}

func (i *iterT) parseComment() {
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

func (i *iterT) matches(oneOf ...string) string {
	startIdx := i.charNr

	lastChecked := ""
	for {
		c, eof := i.checkC(i.charNr)
		if eof {
			i.charNr = startIdx
			return ""
		}
		offset := i.charNr - startIdx

		for idx, key := range oneOf {
			keyLen := uint64(len(key))
			if offset < keyLen {
				if key[offset] != c {
					// Nullify value so we won't check it again
					oneOf[idx] = ""
				} else if keyLen == offset+1 {
					i.charNr++
					return key
				} else {
					lastChecked = key
				}
			}
		}

		if lastChecked == "" {
			i.charNr = startIdx
			return ""
		}

		i.charNr++
	}
}

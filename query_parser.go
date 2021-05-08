package graphql

import (
	"errors"
	"strconv"
	"strings"
)

var (
	ErrorUnexpectedEOF = errors.New("unexpected EOF")
)

type Operator struct {
	operationType       string // "query" || "mutation" || "subscription"
	name                string // "" = no name given
	selection           SelectionSet
	directives          Directives
	variableDefinitions []VariableDefinition
}

type SelectionSet []Selection

type Selection struct {
	selectionType  string          // "Field" || "FragmentSpread" || "InlineFragment"
	field          *Field          // Optional
	fragmentSpread *FragmentSpread // Optional
	inlineFragment *InlineFragment // Optional
}

type Field struct {
	name       string
	alias      string           // Optional
	selection  SelectionSet     // Optional
	directives Directives       // Optional
	arguments  map[string]Value // Optional
}

type FragmentSpread struct {
	name       string
	directives Directives // Optional
}

type InlineFragment struct {
	selection           SelectionSet
	onTypeConditionName string     // Optional
	directives          Directives // Optional
}

type Directives map[string]Directive

type Directive struct {
	name      string
	arguments map[string]Value
}

type TypeReference struct {
	list    bool
	nonNull bool

	// list == false
	name string

	// list == true
	listType *TypeReference
}

type VariableDefinition struct {
	name         string
	varType      TypeReference
	defaultValue *Value
}

type Value struct {
	// "Variable" || "IntValue" || "FloatValue" || "StringValue" || "BooleanValue" || "NullValue" || "EnumValue" || "ListValue" || "ObjectValue"
	valType string

	variable     string
	intValue     int
	floatValue   float64
	stringValue  string
	booleanValue bool
	enumValue    string
	listValue    []Value
	objectValue  map[string]Value
}

func ParseQuery(input string) (*Operator, error) {
	iter := &Iter{
		data: input,
	}

	res, err := iter.parseOperator()
	if err != nil {
		return nil, err
	}

	return res, nil
}

type Iter struct {
	data   string
	charNr uint64
}

func (i *Iter) checkC(nr uint64) (res rune, end bool) {
	if i.eof(nr) {
		return 0, true
	}
	return i.c(nr), false
}

func (i *Iter) c(nr uint64) rune {
	return rune(i.data[nr])
}

func (i *Iter) eof(nr uint64) bool {
	return nr >= uint64(len(i.data))
}

func (i *Iter) currentC() rune {
	return i.c(i.charNr)
}

// https://spec.graphql.org/June2018/#sec-Language.Operations
func (i *Iter) parseOperator() (*Operator, error) {
	res := Operator{
		operationType:       "query",
		name:                "",
		selection:           SelectionSet{},
		directives:          Directives{},
		variableDefinitions: []VariableDefinition{},
	}

	stage := ""

	// This can only return EOF errors atm and as we handle those differently here we can ignore the error
	_ = i.mightIgnoreNextTokens()

	if i.eof(i.charNr) {
		return nil, nil
	}

	switch i.currentC() {
	case '{':
		// For making a query you don't have to define a stage
		// Just continue here
	default:
		newOperationType := i.matches("query", "mutation", "subscription")
		if len(newOperationType) == 0 {
			return nil, errors.New("unknown operation type")
		}
		res.operationType = newOperationType
		stage = "name"

		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
	}

	if stage == "name" {
		switch i.currentC() {
		case '(':
			stage = "variableDefinitions"
		case '@':
			stage = "directives"
		case '{':
			stage = "selectionSets"
		default:
			name, err := i.parseName()
			if err != nil {
				return nil, err
			}
			res.name = name
			stage = "variableDefinitions"

			err = i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}
		}
	}

	if stage == "variableDefinitions" {
		switch i.currentC() {
		case '@':
			stage = "directives"
		case '{':
			stage = "selectionSets"
		case '(':
			i.charNr++
			variableDefinitions, err := i.parseVariableDefinitions()
			if err != nil {
				return nil, err
			}
			res.variableDefinitions = variableDefinitions
			stage = "directives"
			err = i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("unexpected character")
		}
	}

	if stage == "directives" {
		switch i.currentC() {
		case '@':
			directives, err := i.parseDirectives()
			if err != nil {
				return nil, err
			}
			res.directives = directives
		case '{':
			stage = "selectionSets"
		default:
			return nil, errors.New("unexpected character")
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
func (i *Iter) parseVariableDefinitions() ([]VariableDefinition, error) {
	res := []VariableDefinition{}
	for {
		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if i.currentC() == ')' {
			i.charNr++
			return res, nil
		}

		variable, err := i.parseVariableDefinition()
		if err != nil {
			return res, err
		}
		res = append(res, variable)
	}
}

// https://spec.graphql.org/June2018/#VariableDefinition
func (i *Iter) parseVariableDefinition() (VariableDefinition, error) {
	res := VariableDefinition{}

	// Parse var name
	varName, err := i.parseVariable(false)
	if err != nil {
		return res, err
	}
	res.name = varName

	// Parse identifier for switching from var name to var type
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return res, err
	}
	if i.currentC() != ':' {
		return res, errors.New("expected \":\" but got \"" + string(i.currentC()) + "\"")
	}
	i.charNr++

	// Parse variable type
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return res, err
	}
	varType, err := i.parseType()
	if err != nil {
		return res, err
	}
	res.varType = *varType

	// Parse optional default value
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return res, err
	}
	if i.currentC() == '=' {
		i.charNr++
		err = i.mightIgnoreNextTokens()
		if err != nil {
			return res, err
		}

		value, err := i.parseValue()
		if err != nil {
			return res, err
		}
		res.defaultValue = value

		err = i.mightIgnoreNextTokens()
		if err != nil {
			return res, err
		}
	}

	return res, nil
}

// https://spec.graphql.org/June2018/#Value
func (i *Iter) parseValue() (*Value, error) {
	res := Value{}

	switch i.currentC() {
	case '$':
		i.charNr++
		varName, err := i.parseVariable(true)
		if err != nil {
			return nil, err
		}
		res.valType = "Variable"
		res.variable = varName
	case '-', '1', '2', '3', '4', '5', '6', '7', '8', '9', '0':
		val, err := i.parseNumberValue()
		if err != nil {
			return nil, err
		}
		res = *val
	case '"':
		// TODO
		// String > https://spec.graphql.org/June2018/#StringValue
	case '[':
		i.charNr++
		list, err := i.parseListValue()
		if err != nil {
			return nil, err
		}
		res.valType = "ListValue"
		res.listValue = list
	case '{':
		i.charNr++
		values, err := i.parseArgumentsOrObjectValues('}')
		if err != nil {
			return nil, err
		}
		res.objectValue = values
	default:
		name, err := i.parseName()
		if err != nil {
			return nil, err
		}
		switch name {
		case "null":
			res.valType = "NullValue"
		case "true", "false":
			res.valType = "BooleanValue"
			res.booleanValue = name == "true"
		case "":
			return nil, errors.New("invalid value")
		default:
			res.valType = "EnumValue"
			res.enumValue = name
		}
	}

	return &res, nil
}

// https://spec.graphql.org/June2018/#ListValue
func (i *Iter) parseListValue() ([]Value, error) {
	res := []Value{}

	firstLoop := true
	for {
		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
		c := i.currentC()
		if c == ']' {
			i.charNr++
			return res, nil
		}

		if !firstLoop && c == ',' {
			i.charNr++
			err := i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}

			c := i.currentC()
			if c == ']' {
				i.charNr++
				return res, nil
			}
		}

		val, err := i.parseValue()
		if err != nil {
			return nil, err
		}
		res = append(res, *val)
		firstLoop = false
	}
}

// Returns FloatValue or IntValue
// https://spec.graphql.org/June2018/#FloatValue
// https://spec.graphql.org/June2018/#IntValue
func (i *Iter) parseNumberValue() (*Value, error) {
	toMap := func(list string) map[rune]bool {
		res := map[rune]bool{}
		for _, char := range list {
			res[char] = true
		}
		return res
	}
	digit := toMap("0123456789")

	resStr := ""
	res := func(isFloat bool) (*Value, error) {
		if !isFloat {
			// Value is int
			i, err := strconv.Atoi(resStr)
			if err != nil {
				return nil, errors.New("unable to parse int")
			}
			return &Value{
				valType:  "IntValue",
				intValue: i,
			}, nil
		}

		f, err := strconv.ParseFloat(resStr, 64)
		if err != nil {
			return nil, errors.New("unable to parse float")
		}

		return &Value{
			valType:    "FloatValue",
			floatValue: f,
		}, nil
	}

	c := i.currentC()
	if c == '-' {
		resStr += string(c)
		i.charNr++
	}

	// parse integer part
	c, eof := i.checkC(i.charNr)
	if eof {
		return nil, ErrorUnexpectedEOF
	}
	if c == '0' {
		resStr += string(c)
		i.charNr++

		c, eof = i.checkC(i.charNr)
		if eof {
			return nil, ErrorUnexpectedEOF
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
				return nil, ErrorUnexpectedEOF
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
		return nil, errors.New("not a valid int or float")
	}

	// Parse optional float fractional part
	c, eof = i.checkC(i.charNr)
	if eof {
		return nil, ErrorUnexpectedEOF
	}
	if c == '.' {
		resStr += string(c)

		// Tread the first number of the fractional part diffrent as it is required
		i.charNr++
		c, eof = i.checkC(i.charNr)
		if eof {
			return nil, ErrorUnexpectedEOF
		}

		_, ok := digit[c]
		if !ok {
			return nil, errors.New("not a valid float")
		}
		resStr += string(c)

		for {
			i.charNr++
			c, eof = i.checkC(i.charNr)
			if eof {
				return nil, ErrorUnexpectedEOF
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
		return nil, ErrorUnexpectedEOF
	}
	if c != 'e' && c != 'E' {
		// We can assume here the value is a float as the this code can only be reached if the value contains "." or "e" or "E"
		return res(true)
	}
	resStr += string(c)

	i.charNr++
	c, eof = i.checkC(i.charNr)
	if eof {
		return nil, ErrorUnexpectedEOF
	}
	if c == '+' || c == '-' {
		resStr += string(c)

		i.charNr++
		c, eof = i.checkC(i.charNr)
		if eof {
			return nil, ErrorUnexpectedEOF
		}
	}

	_, ok := digit[c]
	if !ok {
		return nil, errors.New("not a valid float")
	}
	resStr += string(c)

	for {
		i.charNr++
		c, eof = i.checkC(i.charNr)
		if eof {
			return nil, ErrorUnexpectedEOF
		}

		_, ok := digit[c]
		if !ok {
			return res(true)
		}
		resStr += string(c)
	}
}

// https://spec.graphql.org/June2018/#Type
func (i *Iter) parseType() (*TypeReference, error) {
	res := TypeReference{}

	if i.currentC() == '[' {
		res.list = true
		i.charNr++
		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		res.listType, err = i.parseType()
		if err != nil {
			return nil, err
		}

		err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
		if i.currentC() != ']' {
			return nil, errors.New("expected list closure (\"]\") but got \"" + string(i.currentC()) + "\"")
		}
		i.charNr++
	} else {
		name, err := i.parseName()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, errors.New("type name missing or invalid type name")
		}
		res.name = name
	}

	err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() == '!' {
		res.nonNull = true
		i.charNr++
	}

	return &res, nil
}

// https://spec.graphql.org/June2018/#Variable
func (i *Iter) parseVariable(alreadyParsedIdentifier bool) (string, error) {
	if !alreadyParsedIdentifier {
		i.mightIgnoreNextTokens()
		if i.currentC() != '$' {
			return "", errors.New("variable must start with \"$\"")
		}
		i.charNr++
	}

	name, err := i.parseName()
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", errors.New("cannot have empty variable name")
	}
	if name == "null" {
		return "", errors.New("null is a illegal variable name")
	}

	return name, nil
}

// https://spec.graphql.org/June2018/#sec-Selection-Sets
func (i *Iter) parseSelectionSets() (SelectionSet, error) {
	res := SelectionSet{}

	for {
		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if i.currentC() == '}' {
			i.charNr++
			return res, nil
		}

		selection, err := i.parseSelection()
		if err != nil {
			return nil, err
		}
		res = append(res, selection)

		err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		switch i.currentC() {
		case ',':
			i.charNr++
		case '}':
			i.charNr++
			return res, nil
		}
	}
}

// https://spec.graphql.org/June2018/#Selection
func (i *Iter) parseSelection() (Selection, error) {
	res := Selection{}

	if len(i.matches("...")) > 0 {
		err := i.mightIgnoreNextTokens()
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
func (i *Iter) parseInlineFragment(hasTypeCondition bool) (*InlineFragment, error) {
	res := InlineFragment{}
	if hasTypeCondition {
		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		res.onTypeConditionName, err = i.parseName()
		if err != nil {
			return nil, err
		}

		if res.onTypeConditionName == "" {
			return nil, errors.New("cannot have type condition without name")
		}
	}

	// parse optional directives
	err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() == '@' {
		res.directives, err = i.parseDirectives()
		if err != nil {
			return nil, err
		}
	}

	// Parse SelectionSet
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() != '{' {
		return nil, errors.New("expected \"{\", not: \"" + string(i.currentC()) + "\"")
	}
	i.charNr++
	res.selection, err = i.parseSelectionSets()
	return &res, err
}

// https://spec.graphql.org/June2018/#FragmentSpread
func (i *Iter) parseFragmentSpread(name string) (*FragmentSpread, error) {
	res := FragmentSpread{name: name}

	// parse optional directives
	err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() == '@' {
		res.directives, err = i.parseDirectives()
		if err != nil {
			return nil, err
		}
	}

	return &res, nil
}

// https://spec.graphql.org/June2018/#Field
func (i *Iter) parseField() (*Field, error) {
	res := Field{}

	// Parse name (and alias if pressent)
	nameOrAlias, err := i.parseName()
	if err != nil {
		return nil, err
	}
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}

	if i.currentC() == ':' {
		if nameOrAlias == "" {
			return nil, errors.New("field alias should have a name")
		}
		res.alias = nameOrAlias
		i.charNr++
		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}
		res.name, err = i.parseName()
		if err != nil {
			return nil, err
		}
		if res.name == "" {
			return nil, errors.New("field should have a name")
		}
	} else {
		if nameOrAlias == "" {
			return nil, errors.New("field should have a name")
		}
		res.name = nameOrAlias
	}

	// Parse Arguments if present
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() == '(' {
		i.charNr++
		args, err := i.parseArgumentsOrObjectValues(')')
		if err != nil {
			return nil, err
		}
		res.arguments = args
	}

	// Parse directives if present
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() == '@' {
		directives, err := i.parseDirectives()
		if err != nil {
			return nil, err
		}
		res.directives = directives
	}

	// Parse SelectionSet if pressent
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() == '{' {
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
func (i *Iter) parseArgumentsOrObjectValues(closure rune) (map[string]Value, error) {
	res := map[string]Value{}

	err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}

	if i.currentC() == closure {
		return res, nil
	}

	for {
		name, err := i.parseName()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, errors.New("argument name must be defined")
		}

		err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if i.currentC() != ':' {
			return nil, errors.New("expected \":\"")
		}
		i.charNr++

		err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		value, err := i.parseValue()
		if err != nil {
			return nil, err
		}
		res[name] = *value

		err = i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if i.currentC() == ',' {
			i.charNr++

			err := i.mightIgnoreNextTokens()
			if err != nil {
				return nil, err
			}
		}

		if i.currentC() == closure {
			i.charNr++
			return res, nil
		}
	}
}

// https://spec.graphql.org/June2018/#Directives
func (i *Iter) parseDirectives() (Directives, error) {
	res := Directives{}
	for {
		err := i.mightIgnoreNextTokens()
		if err != nil {
			return nil, err
		}

		if i.currentC() != '@' {
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
func (i *Iter) parseDirective() (*Directive, error) {
	name, err := i.parseName()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("directive must have a name")
	}
	res := Directive{name: name}

	// Parse optional Arguments
	err = i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() == '(' {
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
func (i *Iter) parseName() (string, error) {
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
			return name, ErrorUnexpectedEOF
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
func (i *Iter) isIgnoredToken() bool {
	c := i.currentC()
	return isUnicodeBom(c) || isWhiteSpace(c) || i.isLineTerminator() || i.isComment(true)
}

func (i *Iter) mightIgnoreNextTokens() error {
	for {
		eof := i.eof(i.charNr)
		if eof {
			return ErrorUnexpectedEOF
		}

		isIgnoredChar := i.isIgnoredToken()
		if !isIgnoredChar {
			return nil
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
func (i *Iter) isLineTerminator() bool {
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
func (i *Iter) isComment(parseComment bool) bool {
	if i.currentC() == '#' {
		if parseComment {
			i.parseComment()
		}
		return true
	}
	return false
}

func (i *Iter) parseComment() {
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

func (i *Iter) matches(oneOf ...string) string {
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
			}
			if keyLen == offset+1 {
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

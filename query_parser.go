package graphql

import (
	"errors"
	"strings"
)

var (
	ErrorUnexpectedEOF = errors.New("unexpected EOF")
)

type Operator struct {
	operationType string // "query" || "mutation" || "subscription"
	name          string // "" = no name given
	selection     SelectionSet
}

type SelectionSet []Selection

type Selection struct {
	selectionType  string          // "Field" || "FragmentSpread" || "InlineFragment"
	field          *Field          // Optional
	fragmentSpread *FragmentSpread // Optional
	inlineFragment *InlineFragment // Optional
}

type Field struct {
	name      string
	alias     string       // Optional
	selection SelectionSet // Optional
}

type FragmentSpread struct {
	name string
}

type InlineFragment struct {
	onTypeConditionName string // Optional
	selection           SelectionSet
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
		operationType: "query",
		name:          "",
		selection:     SelectionSet{},
	}

	stage := "operationType"

	for {
		if i.eof(i.charNr) {
			if stage != "operationType" {
				return nil, ErrorUnexpectedEOF
			}
			return &res, nil
		}

		if i.isIgnoredToken() {
			i.charNr++
			continue
		}

		c := i.currentC()
		switch stage {
		case "operationType":
			switch c {
			// For making a query you don't have to define a stage
			case '{':
				stage = "selectionSets"
			default:
				newOperationType := i.matches("query", "mutation", "subscription")
				if len(newOperationType) > 0 {
					res.operationType = newOperationType
					stage = "name"
					continue
				}
				return nil, errors.New("unknown operation type")
			}
		case "name":
			switch c {
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
			}
		case "variableDefinitions":
			switch c {
			case '@':
				stage = "directives"
			case '{':
				stage = "selectionSets"
			default:
				// TODO: https://spec.graphql.org/June2018/#VariableDefinitions
				return nil, errors.New("https://spec.graphql.org/June2018/#VariableDefinitions")
			}
		case "directives":
			switch c {
			case '{':
				stage = "selectionSets"
			default:
				// TODO: https://spec.graphql.org/June2018/#sec-Language.Directives
				return nil, errors.New("https://spec.graphql.org/June2018/#sec-Language.Directives")
			}
		case "selectionSets":
			selection, err := i.parseSelectionSets()
			if err != nil {
				return nil, err
			}
			res.selection = selection
			return &res, nil
		}

		i.charNr++
	}
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

	// TODO parse optional directives

	// Parse SelectionSet
	err := i.mightIgnoreNextTokens()
	if err != nil {
		return nil, err
	}
	if i.currentC() != '{' {
		return nil, errors.New("expected \"{\"")
	}
	i.charNr++
	res.selection, err = i.parseSelectionSets()
	return &res, err
}

// https://spec.graphql.org/June2018/#FragmentSpread
func (i *Iter) parseFragmentSpread(name string) (*FragmentSpread, error) {
	res := FragmentSpread{name}

	// TODO parse optional Directives

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

	// TODO Next:
	// Arguments (opt)
	// Directives (opt)

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

// func (i *Iter) mustIgnoreNextTokens() error {
// 	ignoredSome := false

// 	for {
// 		eof := i.eof(i.charNr)
// 		if eof {
// 			return ErrorUnexpectedEOF
// 		}

// 		isIgnoredChar := i.isIgnoredToken()
// 		if !isIgnoredChar {
// 			if !ignoredSome {
// 				return errors.New("expected some kind of empty space")
// 			}
// 			return nil
// 		}
// 		ignoredSome = true

// 		i.charNr++
// 	}
// }

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

package graphql

import (
	"fmt"
)

func ParseQueryAndCheckNames(input string) (fragments, operatorsMap map[string]operator, resErrors []error) {
	resErrors = []error{}
	fragments = map[string]operator{}
	operatorsMap = map[string]operator{}

	operators, err := parseQuery(input)
	if err != nil {
		resErrors = append(resErrors, err)
		return
	}
	unknownQueries := 0

	unknownMutations := 0
	unknownSubscriptions := 0

	for _, item := range operators {
		if item.name == "" {
			switch item.operationType {
			case "query":
				unknownQueries++
				item.name = fmt.Sprintf("unknown_query_%d", unknownQueries)
			case "mutation":
				unknownMutations++
				item.name = fmt.Sprintf("unknown_mutation_%d", unknownMutations)
			case "subscription":
				unknownSubscriptions++
				item.name = fmt.Sprintf("unknown_subscription_%d", unknownSubscriptions)
			}
			// "fragment" doesn't have to be handled here as it's required for those to have a name
		}

		if item.operationType == "fragment" {
			_, ok := fragments[item.name]
			if ok {
				resErrors = append(resErrors, fmt.Errorf("fragment name can only be used once (name = \"%s\")", item.name))
				continue
			}

			fragments[item.name] = *item
		} else {
			_, ok := operatorsMap[item.name]
			if ok {
				resErrors = append(resErrors, fmt.Errorf("operator name can only be used once (name = \"%s\")", item.name))
				continue
			}

			operatorsMap[item.name] = *item
		}
	}

	return
}

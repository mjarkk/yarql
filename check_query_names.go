package graphql

import (
	"fmt"
)

func ParseQueryAndCheckNames(input string) (fragments, operatorsMap map[string]Operator, resErrors []error) {
	resErrors = []error{}
	fragments = map[string]Operator{}
	operatorsMap = map[string]Operator{}

	operators, err := ParseQuery(input)
	if err != nil {
		resErrors = append(resErrors, err)
		return
	}
	unkownQueries := 0

	unkownMutations := 0
	unkownSubscriptions := 0

	for _, item := range operators {
		if item.name == "" {
			switch item.operationType {
			case "query":
				unkownQueries++
				item.name = fmt.Sprintf("unknown_query_%d", unkownQueries)
			case "mutation":
				unkownMutations++
				item.name = fmt.Sprintf("unknown_mutation_%d", unkownMutations)
			case "subscription":
				unkownSubscriptions++
				item.name = fmt.Sprintf("unknown_subscription_%d", unkownSubscriptions)
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

package graphql

import (
	"fmt"
	"strconv"
)

func ParseQueryAndCheckNames(input string, ctx *Ctx) (fragments, operatorsMap map[string]operator, resErrors []error) {
	resErrors = []error{}
	fragments = map[string]operator{}
	operatorsMap = map[string]operator{}

	if ctx != nil {
		ctx.startTrace()
	}
	operators, err := parseQuery(input)
	if err != nil {
		resErrors = append(resErrors, err)
		return
	}
	if ctx != nil {
		ctx.finishTrace(func(offset, duration int64) {
			ctx.tracing.Parsing.StartOffset = offset
			ctx.tracing.Parsing.Duration = duration
		})

		ctx.startTrace()
	}

	unknownQueries := 0
	unknownMutations := 0
	unknownSubscriptions := 0

	for _, item := range operators {
		if item.name == "" {
			switch item.operationType {
			case "query":
				unknownQueries++

				item.name = "unknown_query_" + strconv.Itoa(unknownQueries)
			case "mutation":
				unknownMutations++
				item.name = "unknown_mutation_" + strconv.Itoa(unknownMutations)
			case "subscription":
				unknownSubscriptions++
				item.name = "unknown_subscription_" + strconv.Itoa(unknownSubscriptions)
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

	if ctx != nil {
		ctx.finishTrace(func(offset, duration int64) {
			ctx.tracing.Validation.StartOffset = offset
			ctx.tracing.Validation.Duration = duration
		})
	}

	return
}

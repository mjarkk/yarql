package tester

import (
	"encoding/json"

	"github.com/mjarkk/go-graphql"
)

func HasType(s *graphql.Schema, typename string) bool {
	vars := map[string]string{"typename": typename}
	varsJson, _ := json.Marshal(vars)

	query := `query ($typename: String) {
		__type(name: $typename) {kind}
	}`
	errs := s.Resolve([]byte(query), graphql.ResolveOptions{
		NoMeta:    true,
		Variables: string(varsJson),
	})
	if len(errs) != 0 {
		return false
	}

	type Res struct {
		Type *struct {
			Kind string `json:"kind"`
		} `json:"__type"`
	}

	var res Res
	err := json.Unmarshal(s.Result, &res)
	if err != nil {
		return false
	}

	return res.Type != nil
}

package graphql

// Ctx contains all the request information and responses
type Ctx struct {
	fragments  map[string]Operator    // Query fragments
	schema     *Schema                // The Go code schema (grahql schema)
	Values     map[string]interface{} // API User values, user can put all their shitty things in here like poems or tax papers
	directvies []Directives           // Directives stored in ctx
	errors     []error
}

func (ctx *Ctx) HasErrors() bool {
	return len(ctx.errors) > 0
}

func (ctx *Ctx) Errors() []error {
	return ctx.errors
}

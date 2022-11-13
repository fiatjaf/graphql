package graphql

import (
	"context"

	"github.com/fiatjaf/graphql/gqlerrors"
	"github.com/fiatjaf/graphql/language/ast"
	"github.com/fiatjaf/graphql/language/parser"
	"github.com/fiatjaf/graphql/language/source"
)

type Params struct {
	// The GraphQL type system to use when validating and executing a query.
	Schema Schema

	// A GraphQL language formatted string representing the requested operation.
	RequestString string

	// The value provided as the first argument to resolver functions on the top
	// level type (e.g. the query object type).
	RootObject map[string]interface{}

	// A mapping of variable name to runtime value to use for all variables
	// defined in the requestString.
	VariableValues map[string]interface{}

	// The name of the operation to use if requestString contains multiple
	// possible operations. Can be omitted if requestString contains only
	// one operation.
	OperationName string

	// Context may be provided to pass application-specific per-request
	// information to resolve functions.
	Context context.Context
}

// DoChannel performs both sync and asynchronous operations (subscriptions), it returns a channel
// of results instead of a single result
func DoAsync(p Params) chan *Result {
	return do(p, false)
}

// Do executes synchronous operations, ignores subscriptions
func Do(p Params) *Result {
	ch := do(p, true)
	return <-ch
}

func do(p Params, skipSubscriptions bool) chan *Result {
	source := source.NewSource(&source.Source{
		Body: []byte(p.RequestString),
		Name: "GraphQL request",
	})

	wrapErr := func(gqlerr gqlerrors.FormattedErrors) chan *Result {
		singleEventChannel := make(chan *Result)
		go func() {
			singleEventChannel <- &Result{Errors: gqlerr}
		}()
		return singleEventChannel
	}

	// run init on the extensions
	extErrs := handleExtensionsInits(&p)
	if len(extErrs) != 0 {
		return wrapErr(extErrs)
	}

	extErrs, parseFinishFn := handleExtensionsParseDidStart(&p)
	if len(extErrs) != 0 {
		return wrapErr(extErrs)
	}

	// parse the source
	AST, err := parser.Parse(parser.ParseParams{Source: source})
	if err != nil {
		// run parseFinishFuncs for extensions
		extErrs = parseFinishFn(err)

		// merge the errors from extensions and the original error from parser
		extErrs = append(extErrs, gqlerrors.FormatErrors(err)...)
		return wrapErr(extErrs)
	}

	// run parseFinish functions for extensions
	extErrs = parseFinishFn(err)
	if len(extErrs) != 0 {
		return wrapErr(extErrs)
	}

	// notify extensions about the start of the validation
	extErrs, validationFinishFn := handleExtensionsValidationDidStart(&p)
	if len(extErrs) != 0 {
		return wrapErr(extErrs)
	}

	// validate document
	validationResult := ValidateDocument(&p.Schema, AST, nil)

	if !validationResult.IsValid {
		// run validation finish functions for extensions
		extErrs = validationFinishFn(validationResult.Errors)

		// merge the errors from extensions and the original error from parser
		extErrs = append(extErrs, validationResult.Errors...)
		return wrapErr(extErrs)
	}

	// run the validationFinishFuncs for extensions
	extErrs = validationFinishFn(validationResult.Errors)
	if len(extErrs) != 0 {
		return wrapErr(extErrs)
	}

	params := ExecuteParams{
		Schema:        p.Schema,
		Root:          p.RootObject,
		AST:           AST,
		OperationName: p.OperationName,
		Args:          p.VariableValues,
		Context:       p.Context,
	}

	if !skipSubscriptions &&
		len(AST.Definitions) > 0 &&
		AST.Definitions[0].(*ast.OperationDefinition).Operation == "subscription" {
		return ExecuteSubscription(params)
	} else {
		singleEventChannel := make(chan *Result)
		go func() {
			singleEventChannel <- Execute(params)
		}()
		return singleEventChannel
	}
}

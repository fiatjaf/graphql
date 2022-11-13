# graphql [![CircleCI](https://circleci.com/gh/fiatjaf/graphql/tree/master.svg?style=svg)](https://circleci.com/gh/fiatjaf/graphql/tree/master) [![Go Reference](https://pkg.go.dev/badge/github.com/fiatjaf/graphql.svg)](https://pkg.go.dev/github.com/fiatjaf/graphql) [![Coverage Status](https://coveralls.io/repos/github/fiatjaf/graphql/badge.svg?branch=master)](https://coveralls.io/github/fiatjaf/graphql?branch=master)

An implementation of GraphQL in Go. Follows the official reference implementation [`graphql-js`](https://github.com/graphql/graphql-js).

Supports: queries, mutations & subscriptions.

### Documentation

godoc: https://pkg.go.dev/github.com/fiatjaf/graphql

### Getting Started

To install the library, run:
```bash
go get github.com/fiatjaf/graphql
```

The following is a simple example which defines a schema with a single `hello` string-type field and a `Resolve` method which returns the string `world`. A GraphQL query is performed against this schema with the resulting output printed in JSON format.

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/fiatjaf/graphql"
)

func main() {
	// Schema
	fields := graphql.Fields{
		"hello": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return "world", nil
			},
		},
	}
	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: fields}
	schemaConfig := graphql.SchemaConfig{Query: graphql.NewObject(rootQuery)}
	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		log.Fatalf("failed to create new schema, error: %v", err)
	}

	// Query
	query := `
		{
			hello
		}
	`
	params := graphql.Params{Schema: schema, RequestString: query}
	r := graphql.Do(params)
	if len(r.Errors) > 0 {
		log.Fatalf("failed to execute graphql operation, errors: %+v", r.Errors)
	}
	rJSON, _ := json.Marshal(r)
	fmt.Printf("%s \n", rJSON) // {"data":{"hello":"world"}}
}
```
For more complex examples, refer to the [examples/](https://github.com/fiatjaf/graphql/tree/master/examples/) directory and [graphql_test.go](https://github.com/fiatjaf/graphql/blob/master/graphql_test.go).

# fiatjaf-handler [![CircleCI](https://circleci.com/gh/fiatjaf/handler.svg?style=svg)](https://circleci.com/gh/fiatjaf/handler) [![GoDoc](https://godoc.org/fiatjaf/handler?status.svg)](https://godoc.org/github.com/fiatjaf/graphql/handler) [![Coverage Status](https://coveralls.io/repos/fiatjaf/handler/badge.svg?branch=master&service=github)](https://coveralls.io/github/fiatjaf/handler?branch=master) [![Join the chat at https://gitter.im/fiatjaf/graphql](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/fiatjaf/graphql?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)


Golang HTTP.Handler for [graphl-go](https://github.com/fiatjaf/graphql)

### Usage

```go
package main

import (
	"net/http"
	"github.com/fiatjaf/handler"
)

func main() {
	schema, _ := graphql.NewSchema(...)

	h := handler.New(&handler.Config{
		Schema: &schema,
		Pretty: true,
		GraphiQL: true,
	})

	http.Handle("/graphql", h)
	http.ListenAndServe(":8080", nil)
}
```

### Using Playground
```go
h := handler.New(&handler.Config{
	Schema: &schema,
	Pretty: true,
	GraphiQL: false,
	Playground: true,
})
```

### Details

The handler will accept requests with
the parameters:

  * **`query`**: A string GraphQL document to be executed.

  * **`variables`**: The runtime values to use for any GraphQL query variables
    as a JSON object.

  * **`operationName`**: If the provided `query` contains multiple named
    operations, this specifies which operation should be executed. If not
    provided, an 400 error will be returned if the `query` contains multiple
    named operations.

GraphQL will first look for each parameter in the URL's query-string:

```
/graphql?query=query+getUser($id:ID){user(id:$id){name}}&variables={"id":"4"}
```

If not found in the query-string, it will look in the POST request body.
The `handler` will interpret it
depending on the provided `Content-Type` header.

  * **`application/json`**: the POST body will be parsed as a JSON
    object of parameters.

  * **`application/x-www-form-urlencoded`**: this POST body will be
    parsed as a url-encoded string of key-value pairs.

  * **`application/graphql`**: The POST body will be parsed as GraphQL
    query string, which provides the `query` parameter.


### Examples
- [golang-graphql-playground](https://github.com/fiatjaf/playground)
- [golang-relay-starter-kit](https://github.com/sogko/golang-relay-starter-kit)
- [todomvc-relay-go](https://github.com/sogko/todomvc-relay-go)

### Test
```bash
$ go get github.com/fiatjaf/handler
$ go build && go test ./...

package handler

import (
	"context"
	"net/http"

	"github.com/fiatjaf/graphql"
	"github.com/fiatjaf/graphql/gqlerrors"
)

const (
	ContentTypeJSON           = "application/json"
	ContentTypeGraphQL        = "application/graphql"
	ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
)

type ResultCallbackFn func(ctx context.Context, params *graphql.Params, result *graphql.Result, responseBody []byte)

type Handler struct {
	Schema           *graphql.Schema
	pretty           bool
	graphiql         bool
	playground       bool
	websocket        bool
	rootObjectFn     RootObjectFn
	resultCallbackFn ResultCallbackFn
	formatErrorFn    func(err error) gqlerrors.FormattedError
}

type RequestOptions struct {
	Query         string                 `json:"query" url:"query" schema:"query"`
	Variables     map[string]interface{} `json:"variables" url:"variables" schema:"variables"`
	OperationName string                 `json:"operationName" url:"operationName" schema:"operationName"`
}

// a workaround for getting`variables` as a JSON string
type requestOptionsCompatibility struct {
	Query         string `json:"query" url:"query" schema:"query"`
	Variables     string `json:"variables" url:"variables" schema:"variables"`
	OperationName string `json:"operationName" url:"operationName" schema:"operationName"`
}

// ServeHTTP provides an entrypoint into executing graphQL queries.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" && h.websocket {
		h.ContextWebsocketHandler(context.Background(), w, r)
	} else {
		h.ContextHandler(r.Context(), w, r)
	}
}

// RootObjectFn allows a user to generate a RootObject per request
type RootObjectFn func(ctx context.Context, r *http.Request) map[string]interface{}

type Config struct {
	Schema           *graphql.Schema
	Pretty           bool
	GraphiQL         bool
	Playground       bool
	WebSocket        bool
	RootObjectFn     RootObjectFn
	ResultCallbackFn ResultCallbackFn
	FormatErrorFn    func(err error) gqlerrors.FormattedError
}

func NewConfig() *Config {
	return &Config{
		Schema:     nil,
		Pretty:     true,
		GraphiQL:   false,
		Playground: true,
		WebSocket:  true,
	}
}

func New(p *Config) *Handler {
	if p == nil {
		p = NewConfig()
	}

	if p.Schema == nil {
		panic("undefined GraphQL schema")
	}

	return &Handler{
		Schema:           p.Schema,
		pretty:           p.Pretty,
		graphiql:         p.GraphiQL,
		websocket:        p.WebSocket,
		playground:       p.Playground,
		rootObjectFn:     p.RootObjectFn,
		resultCallbackFn: p.ResultCallbackFn,
		formatErrorFn:    p.FormatErrorFn,
	}
}

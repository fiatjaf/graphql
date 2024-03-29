package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	syncmap "github.com/SaveTheRbtz/generic-sync-map-go"
	"github.com/fiatjaf/graphql"
	"github.com/fiatjaf/graphql/gqlerrors"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = pongWait / 2

	// Maximum message size allowed from peer.
	maxMessageSize = 512000
)

type WebSocket struct {
	conn                   *websocket.Conn
	mutex                  sync.Mutex
	subscriptionCancellers syncmap.MapOf[string, context.CancelFunc]
}

func (ws *WebSocket) WriteJSON(any interface{}) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	return ws.conn.WriteJSON(any)
}

func (ws *WebSocket) WriteMessage(t int, b []byte) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	return ws.conn.WriteMessage(t, b)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
	Subprotocols:    []string{"graphql-ws", "graphql-transport-ws"},
}

type GraphQLWSMessage struct {
	ID      any             `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type GraphQLWSSubscriptionPayload struct {
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
	Extensions    map[string]any `json:"extensions"`
}

// ContextHandler provides an entrypoint into executing graphQL queries and subscriptions with
// user-provided context.
func (h *Handler) ContextWebsocketHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket: %s", err.Error())
		return
	}
	ticker := time.NewTicker(pingPeriod)
	ws := &WebSocket{conn: conn}

	terminateConnection := func() {
		ticker.Stop()
		conn.Close()

		ws.subscriptionCancellers.Range(func(id string, cancel context.CancelFunc) bool {
			ws.subscriptionCancellers.Delete(id)
			cancel()
			return true
		})
	}

	// reader
	go func() {
		defer terminateConnection()

		conn.SetReadLimit(maxMessageSize)
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		for {
			typ, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(
					err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("unexpected close error: %s", err.Error())
				} else {
					log.Printf("error on read: %s", err.Error())
				}
				break
			}

			if typ == websocket.PingMessage {
				ws.WriteMessage(websocket.PongMessage, nil)
				continue
			}

			go func(message []byte) {
				var msg GraphQLWSMessage
				err := json.Unmarshal(message, &msg)
				if err != nil {
					b, _ := json.Marshal(err.Error())
					ws.WriteJSON(GraphQLWSMessage{Type: "error", Payload: b})
					return
				}

				switch msg.Type {
				case "connection_init":
					ws.WriteJSON(GraphQLWSMessage{Type: "connection_ack"})

					// clients may send headers in this object, we can use this to modify the context
					// this works because "connection_init" is always the first message
					if h.ModifyContextOnHeaders != nil {
						var headers map[string]string
						if err := json.Unmarshal(msg.Payload, &headers); err == nil {
							ctx = h.ModifyContextOnHeaders(ctx, headers)
						}
					}

				case "subscribe", "start":
					// this will be "subscribe" for graphiql and "start" for playground and zebedee-app
					dataMessageName, _ := map[string]string{
						"subscribe": "next",
						"start":     "data",
					}[msg.Type]

					var payload GraphQLWSSubscriptionPayload
					err := json.Unmarshal(msg.Payload, &payload)
					if err != nil {
						b, _ := json.Marshal(err.Error())
						ws.WriteJSON(GraphQLWSMessage{Type: "error", Payload: b})
						return
					}

					cancellableCtx, cancel := context.WithCancel(ctx)
					ws.subscriptionCancellers.Store(fmt.Sprintf("%v", msg.ID), cancel)

					params := graphql.Params{
						Schema:         *h.Schema,
						RequestString:  payload.Query,
						VariableValues: payload.Variables,
						OperationName:  payload.OperationName,
						Context:        cancellableCtx,
					}

					writeResult := func(result *graphql.Result) {
						b, _ := json.Marshal(result)
						ws.WriteJSON(GraphQLWSMessage{
							ID:      msg.ID,
							Payload: json.RawMessage(b),

							// this will be "next" for graphiql and "data" for graphql-playground
							Type: dataMessageName,
						})
					}

					if strings.HasPrefix(strings.TrimLeft(payload.Query, " "), "subscription") {
						// subscription
						ch := graphql.DoAsync(params)
						for result := range ch {
							writeResult(result)
						}
					} else {
						// query or mutation
						result := graphql.Do(params)
						if formatErrorFn := h.formatErrorFn; formatErrorFn != nil && len(result.Errors) > 0 {
							formatted := make([]gqlerrors.FormattedError, len(result.Errors))
							for i, formattedError := range result.Errors {
								formatted[i] = formatErrorFn(formattedError.OriginalError())
							}
							result.Errors = formatted
						}
						writeResult(result)
						cancel() // cancel the context here
					}

				case "stop":
					// cancel the context for this subscription such that we stop streaming graphql data into nowhere
					if cancel, ok := ws.subscriptionCancellers.Load(fmt.Sprintf("%v", msg.ID)); ok {
						ws.subscriptionCancellers.Delete(fmt.Sprintf("%v", msg.ID))
						cancel()
					}
				}
			}(message)
		}
	}()

	// writer
	go func() {
		defer terminateConnection()

		for {
			select {
			case <-ticker.C:
				err := ws.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					log.Printf("error writing ping, closing websocket: %s", err.Error())
					return
				}
			}
		}
	}()
}

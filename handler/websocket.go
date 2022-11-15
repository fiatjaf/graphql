package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/fiatjaf/graphql"
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
	conn  *websocket.Conn
	mutex sync.Mutex
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
	Subprotocols:    []string{"graphql-ws"},
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

		h.subscriptionCancellers.Range(func(id string, cancel context.CancelFunc) bool {
			h.subscriptionCancellers.Delete(id)
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
				case "start":
					var payload GraphQLWSSubscriptionPayload
					err := json.Unmarshal(msg.Payload, &payload)
					if err != nil {
						b, _ := json.Marshal(err.Error())
						ws.WriteJSON(GraphQLWSMessage{Type: "error", Payload: b})
						return
					}

					cancellableCtx, cancel := context.WithCancel(ctx)
					h.subscriptionCancellers.Store(fmt.Sprintf("%v", msg.ID), cancel)

					params := graphql.Params{
						Schema:         *h.Schema,
						RequestString:  payload.Query,
						VariableValues: payload.Variables,
						OperationName:  payload.OperationName,
						Context:        cancellableCtx,
					}
					ch := graphql.DoAsync(params)
					for result := range ch {
						b, _ := json.Marshal(result)
						ws.WriteJSON(GraphQLWSMessage{
							ID:      msg.ID,
							Type:    "data",
							Payload: json.RawMessage(b),
						})
					}
				case "stop":
					// cancel the context for this subscription such that we stop streaming graphql data into nowhere
					if cancel, ok := h.subscriptionCancellers.Load(fmt.Sprintf("%v", msg.ID)); ok {
						h.subscriptionCancellers.Delete(fmt.Sprintf("%v", msg.ID))
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

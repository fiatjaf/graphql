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
	ID      string          `json:"id"`
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

	// reader
	go func() {
		defer func() {
			ticker.Stop()
			conn.Close()
		}()

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
				}
				break
			}

			if typ == websocket.PingMessage {
				ws.WriteMessage(websocket.PongMessage, nil)
				continue
			}

			go func(message []byte) {
				var msg GraphQLWSMessage
				fmt.Println("MESSAGE:", string(message))
				err := json.Unmarshal(message, &msg)
				if err != nil {
					ws.WriteJSON(GraphQLWSMessage{Type: "error"})
					return
				}

				switch msg.Type {
				case "connection_init":
					ws.WriteJSON(GraphQLWSMessage{Type: "connection_ack"})
				case "start", "subscribe":
					var payload GraphQLWSSubscriptionPayload
					err := json.Unmarshal(msg.Payload, &payload)
					if err != nil {
						ws.WriteJSON(GraphQLWSMessage{Type: "error"})
						return
					}

					params := graphql.Params{
						Schema:         *h.Schema,
						RequestString:  payload.Query,
						VariableValues: payload.Variables,
						OperationName:  payload.OperationName,
						Context:        ctx,
					}
					ch := graphql.DoAsync(params)
					for result := range ch {
						fmt.Println("RESULT:", result.Data)
						b, _ := json.Marshal(result)
						ws.WriteJSON(GraphQLWSMessage{
							ID:      msg.ID,
							Type:    "data",
							Payload: json.RawMessage(b),
						})
					}
				}
			}(message)
		}
	}()

	// writer
	go func() {
		defer func() {
			ticker.Stop()
			conn.Close()
		}()

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

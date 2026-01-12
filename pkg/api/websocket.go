// Package api provides WebSocket support for real-time communication.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// Event represents a WebSocket event.
type Event struct {
	Type      string    `json:"type"`
	AgentID   string    `json:"agent_id,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// WebSocketHub manages WebSocket connections.
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan Event
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
}

// WebSocketClient represents a connected client.
type WebSocketClient struct {
	hub        *WebSocketHub
	conn       *websocket.Conn
	send       chan Event
	subscriptions map[string]bool // topic -> subscribed
	mu         sync.RWMutex
}

// NewWebSocketHub creates a new WebSocket hub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// Run starts the hub's event loop.
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			event.Timestamp = time.Now()
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- event:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends an event to all clients.
func (h *WebSocketHub) Broadcast(event Event) {
	select {
	case h.broadcast <- event:
	default:
		log.Println("broadcast channel full, dropping event")
	}
}

// BroadcastToAgent sends an event to clients subscribed to an agent.
func (h *WebSocketHub) BroadcastToAgent(agentID string, event Event) {
	event.AgentID = agentID
	event.Timestamp = time.Now()

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		client.mu.RLock()
		subscribed := client.subscriptions["agent:"+agentID] || client.subscriptions["*"]
		client.mu.RUnlock()

		if subscribed {
			select {
			case client.send <- event:
			default:
			}
		}
	}
}

// handleWebSocket handles WebSocket connections.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	websocket.Handler(func(conn *websocket.Conn) {
		client := &WebSocketClient{
			hub:           s.hub,
			conn:          conn,
			send:          make(chan Event, 256),
			subscriptions: make(map[string]bool),
		}

		s.hub.register <- client

		// Send welcome message
		client.send <- Event{
			Type: "connected",
			Data: map[string]string{"message": "Connected to GoFlow"},
		}

		// Start writer goroutine
		go client.writePump()

		// Reader loop
		client.readPump(s)
	}).ServeHTTP(w, r)
}

// writePump sends events to the client.
func (c *WebSocketClient) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for event := range c.send {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}

		if _, err := c.conn.Write(data); err != nil {
			return
		}
	}
}

// ClientMessage is a message from the client.
type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// SubscribePayload is the payload for subscribe/unsubscribe.
type SubscribePayload struct {
	Topics []string `json:"topics"`
}

// ChannelPayload is the payload for channel messages.
type ChannelPayload struct {
	Channel string `json:"channel"`
	Topic   string `json:"topic"`
	Data    any    `json:"data"`
}

// readPump reads messages from the client.
func (c *WebSocketClient) readPump(s *Server) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		var msg ClientMessage
		if err := websocket.JSON.Receive(c.conn, &msg); err != nil {
			return
		}

		switch msg.Type {
		case "subscribe":
			var payload SubscribePayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}
			c.mu.Lock()
			for _, topic := range payload.Topics {
				c.subscriptions[topic] = true
			}
			c.mu.Unlock()

			c.send <- Event{
				Type: "subscribed",
				Data: map[string]any{"topics": payload.Topics},
			}

		case "unsubscribe":
			var payload SubscribePayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}
			c.mu.Lock()
			for _, topic := range payload.Topics {
				delete(c.subscriptions, topic)
			}
			c.mu.Unlock()

		case "publish":
			var payload ChannelPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}

			s.hub.Broadcast(Event{
				Type: "channel.message",
				Data: map[string]any{
					"channel": payload.Channel,
					"topic":   payload.Topic,
					"data":    payload.Data,
				},
			})

		case "ping":
			c.send <- Event{Type: "pong"}
		}
	}
}

// ConnectionCount returns the number of connected clients.
func (h *WebSocketHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

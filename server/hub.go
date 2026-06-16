package server

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"

	"aispulse/store"
)

// WSMessage is sent over WebSocket to the browser.
type WSMessage struct {
	Type   string        `json:"type"`
	Vessel *store.Vessel `json:"vessel,omitempty"`
	Raw    string        `json:"raw,omitempty"`
}

// client wraps a single WebSocket connection.
type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub manages all connected WebSocket clients and broadcasts messages.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// Broadcast sends a message to every connected client.
func (h *Hub) Broadcast(vessel *store.Vessel, raw string) {
	msg := WSMessage{Type: "vessel_update", Vessel: vessel, Raw: raw}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			log.Printf("[hub] client buffer full — dropping message")
		}
	}
}

// writePump drains the send channel to the WebSocket connection.
func (c *client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

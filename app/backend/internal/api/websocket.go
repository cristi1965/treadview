package api

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // allow all origins for dev
}

// Hub manages WebSocket connections and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]bool),
	}
}

// HandleWebSocket upgrades HTTP to WebSocket and registers the connection.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	log.Printf("[WS] Client connected (%d total)", len(h.clients))

	// Keep connection alive; remove on disconnect
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
			log.Printf("[WS] Client disconnected (%d remaining)", len(h.clients))
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// Broadcast sends a JSON message to all connected clients.
func (h *Hub) Broadcast(msg any) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.clients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("[WS] Write error: %v", err)
			conn.Close()
			// Don't modify map during iteration; cleanup happens in reader goroutine
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

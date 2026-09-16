package api

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     websocketOriginAllowed,
}

const websocketWriteTimeout = 5 * time.Second

type websocketClient struct {
	conn      *websocket.Conn
	send      chan any
	done      chan struct{}
	closeOnce sync.Once
}

func websocketOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if origin == "wails://wails" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && strings.EqualFold(parsed.Host, r.Host)
}

// Hub manages WebSocket connections and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]*websocketClient
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]*websocketClient),
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
	client := &websocketClient{conn: conn, send: make(chan any, 32), done: make(chan struct{})}
	h.clients[conn] = client
	count := len(h.clients)
	h.mu.Unlock()

	log.Printf("[WS] Client connected (%d total)", count)

	go h.writeLoop(client)

	// Keep connection alive; remove on disconnect.
	go func() {
		defer h.removeClient(client)
		conn.SetReadLimit(64 << 10)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

func (h *Hub) writeLoop(client *websocketClient) {
	for {
		select {
		case <-client.done:
			return
		case msg := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(websocketWriteTimeout))
			if err := client.conn.WriteJSON(msg); err != nil {
				log.Printf("[WS] Write error: %v", err)
				h.removeClient(client)
				return
			}
		}
	}
}

func (h *Hub) removeClient(client *websocketClient) {
	if client == nil {
		return
	}
	client.closeOnce.Do(func() {
		close(client.done)
		h.mu.Lock()
		delete(h.clients, client.conn)
		count := len(h.clients)
		h.mu.Unlock()
		_ = client.conn.Close()
		log.Printf("[WS] Client disconnected (%d remaining)", count)
	})
}

// Broadcast sends a JSON message to all connected clients.
func (h *Hub) Broadcast(msg any) {
	h.mu.RLock()
	clients := make([]*websocketClient, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- msg:
		case <-client.done:
		default:
			log.Printf("[WS] Client send queue full; disconnecting slow client")
			h.removeClient(client)
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

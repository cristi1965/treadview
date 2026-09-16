package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"trading-agents/internal/config"
)

func TestCORSRejectsNullOrigin(t *testing.T) {
	router := SetupRouter(&config.Config{AdminToken: "secret"}, nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	req.Header.Set("Origin", "null")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("null origin was allowed: %q", got)
	}
}

func TestHubSerializesConcurrentBroadcasts(t *testing.T) {
	hub := NewHub()
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()
	header := http.Header{}
	header.Set("Origin", server.URL)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	const messages = 16
	var wg sync.WaitGroup
	for i := 0; i < messages; i++ {
		wg.Add(1)
		go func(value int) {
			defer wg.Done()
			hub.Broadcast(map[string]int{"value": value})
		}(i)
	}
	wg.Wait()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	for i := 0; i < messages; i++ {
		var payload map[string]int
		if err := conn.ReadJSON(&payload); err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
	}
}

func TestWebSocketOriginPolicy(t *testing.T) {
	same := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/ws", nil)
	same.Host = "127.0.0.1:8765"
	same.Header.Set("Origin", "http://127.0.0.1:8765")
	if !websocketOriginAllowed(same) {
		t.Fatal("same-origin websocket rejected")
	}
	cross := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/ws", nil)
	cross.Host = "127.0.0.1:8765"
	cross.Header.Set("Origin", "https://attacker.example")
	if websocketOriginAllowed(cross) {
		t.Fatal("cross-origin websocket allowed")
	}
}

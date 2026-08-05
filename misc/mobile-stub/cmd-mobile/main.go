package mobile

import (
	"log"
	"trading-agents/internal/api"
	"trading-agents/internal/config"
	"trading-agents/internal/llm"
	"trading-agents/internal/orchestrator"
)

// StartBackend runs the Go server in a blocking loop.
// It is designed to be called on a background thread by iOS (via gomobile bind)
// or Android.
func StartBackend(apiKey string) {
	log.Println("[Go Mobile] Starting embedded Go backend on background thread...")
	
	// Load configuration
	cfg := config.Load()
	cfg.Port = "8765" // Lock to local port
	if apiKey != "" {
		cfg.GoogleAPIKey = apiKey
	}

	if cfg.GoogleAPIKey == "" {
		log.Println("[Go Mobile] ERROR: Google API key not set")
		return
	}

	// Initialize LLM client
	geminiClient, err := llm.NewGeminiClient(cfg)
	if err != nil {
		log.Printf("[Go Mobile] Failed to initialize Gemini: %v", err)
		return
	}

	// Create orchestrator
	orch := orchestrator.New(cfg, geminiClient)

	// Setup API router
	router := api.SetupRouter(cfg, orch)

	log.Println("[Go Mobile] Embedded HTTP/WS server listening on 127.0.0.1:8765")
	if err := router.Run("127.0.0.1:8765"); err != nil {
		log.Printf("[Go Mobile] Embedded server shut down: %v", err)
	}
}

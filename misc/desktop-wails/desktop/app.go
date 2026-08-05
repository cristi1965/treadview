package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"trading-agents/internal/api"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/llm"
	"trading-agents/internal/orchestrator"
)

type DesktopApp struct {
	apiBaseURL string
	server     *http.Server
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{
		apiBaseURL: "http://127.0.0.1:8765",
	}
}

func (a *DesktopApp) startup(ctx context.Context) {
	if err := a.startBackend(); err != nil {
		log.Fatalf("failed to initialize desktop backend: %v", err)
	}
	log.Printf("desktop backend ready at %s", a.apiBaseURL)
}

func (a *DesktopApp) startBackend() error {
	if err := configureBackendWorkingDir(); err != nil {
		log.Printf("[WARN] Desktop working directory not changed: %v", err)
	}

	dataDir, err := desktopDataDir()
	if err != nil {
		return err
	}
	database.InitDB(dataDir)

	cfg := config.Load()
	cfg.Port = "8765"

	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		log.Printf("[WARN] Desktop LLM disabled: %v", err)
		llmClient = llm.NewUnavailableClient(err.Error())
	}

	router := api.SetupRouter(cfg, orchestrator.New(cfg, llmClient))
	listener, err := net.Listen("tcp", "127.0.0.1:"+cfg.Port)
	if err != nil {
		return fmt.Errorf("listen on 127.0.0.1:%s: %w", cfg.Port, err)
	}

	server := &http.Server{Handler: router}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("desktop backend stopped unexpectedly: %v", err)
		}
	}()

	a.apiBaseURL = "http://127.0.0.1:" + cfg.Port
	a.server = server
	return nil
}

func (a *DesktopApp) shutdown(ctx context.Context) {
	if a.server == nil {
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		log.Printf("desktop backend shutdown failed: %v", err)
	}
}

func (a *DesktopApp) GetAPIBaseURL() string {
	return a.apiBaseURL
}

func desktopDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	dir := filepath.Join(base, "TradingAgents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create desktop data dir: %w", err)
	}
	return dir, nil
}

func configureBackendWorkingDir() error {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("resolve desktop source path")
	}

	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	if _, err := os.Stat(filepath.Join(backendRoot, "data")); err != nil {
		return fmt.Errorf("backend data dir unavailable at %s: %w", backendRoot, err)
	}
	if err := os.Chdir(backendRoot); err != nil {
		return fmt.Errorf("chdir to backend root %s: %w", backendRoot, err)
	}
	log.Printf("desktop working directory: %s", backendRoot)
	return nil
}

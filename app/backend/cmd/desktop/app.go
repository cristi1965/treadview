package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"trading-agents/internal/api"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/llm"
	"trading-agents/internal/market"
	"trading-agents/internal/orchestrator"
)

type DesktopApp struct {
	apiBaseURL        string
	server            *http.Server
	stopScanner       *api.PaperStopScanner
	stopScannerCancel context.CancelFunc
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

	cfg := config.Load()
	cfg.Port = "8765"
	database.InitDB(cfg.DatabaseDir)

	market.Default().StartLiveRefresh()

	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		log.Printf("[WARN] Desktop LLM disabled: %v", err)
		llmClient = llm.NewUnavailableClient(err.Error())
	}

	scannerCtx, cancelScanner := context.WithCancel(context.Background())
	stopScanner := api.NewPaperStopScanner(database.DB, cfg)
	if err := stopScanner.Start(scannerCtx); err != nil {
		cancelScanner()
		return fmt.Errorf("start local Paper stop scanner: %w", err)
	}
	router := api.SetupRouterWithPaperStopScanner(cfg, orchestrator.New(cfg, llmClient), stopScanner)
	listener, err := net.Listen("tcp", "127.0.0.1:"+cfg.Port)
	if err != nil {
		cancelScanner()
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
	a.stopScanner = stopScanner
	a.stopScannerCancel = cancelScanner
	return nil
}

func (a *DesktopApp) shutdown(ctx context.Context) {
	if a.stopScannerCancel != nil {
		a.stopScannerCancel()
	}
	if a.stopScanner != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()
		if err := a.stopScanner.Stop(stopCtx); err != nil {
			log.Printf("desktop Paper stop scanner shutdown failed: %v", err)
		}
	}
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

// Notify shows a native desktop notification (macOS via osascript; others log).
func (a *DesktopApp) Notify(title, body string) {
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		if err := exec.Command("osascript", "-e", script).Start(); err != nil {
			log.Printf("[desktop] notify failed: %v", err)
		}
		return
	}
	log.Printf("[desktop] notify: %s — %s", title, body)
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

package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	// Server
	Port string

	// LLM Provider
	LLMProvider    string
	DeepThinkLLM   string
	QuickThinkLLM  string
	LLMBackendURL  string
	Temperature    float64
	HasTemperature bool

	// Gemini
	GoogleAPIKey  string
	ThinkingLevel string

	// DeepSeek
	DeepSeekAPIKey string

	// OpenAI
	OpenAIAPIKey string

	// OpenAI-compatible (for generic endpoints)
	OpenAICompatAPIKey string

	// Data sources
	FREDAPIKey string

	// Analysis
	MaxDebateRounds      int
	MaxRiskDiscussRounds int
	MaxRecurLimit        int
	OutputLanguage       string

	// Paths
	ResultsDir   string
	DataCacheDir string
}

// Load reads configuration from environment and .env files.
func Load() *Config {
	// Try loading .env from project root (two levels up from backend)
	_ = godotenv.Load("../../.env")
	// Also try local .env
	_ = godotenv.Load(".env")

	cfg := &Config{
		Port:              getEnv("PORT", "8765"),
		LLMProvider:       getEnv("TRADINGAGENTS_LLM_PROVIDER", "google"),
		LLMBackendURL:     getEnv("TRADINGAGENTS_LLM_BACKEND_URL", ""),
		DeepThinkLLM:      getEnv("TRADINGAGENTS_DEEP_THINK_LLM", "gemini-3.5-flash"),
		QuickThinkLLM:     getEnv("TRADINGAGENTS_QUICK_THINK_LLM", "gemini-3.1-flash-lite"),
		ThinkingLevel:     getEnv("TRADINGAGENTS_GOOGLE_THINKING_LEVEL", ""),

		// API Keys
		GoogleAPIKey:      getEnv("GOOGLE_API_KEY", ""),
		DeepSeekAPIKey:    getEnv("DEEPSEEK_API_KEY", ""),
		OpenAIAPIKey:      getEnv("OPENAI_API_KEY", ""),
		OpenAICompatAPIKey: getEnv("OPENAI_COMPATIBLE_API_KEY", ""),

		FREDAPIKey: getEnv("FRED_API_KEY", ""),

		MaxDebateRounds:      getEnvInt("TRADINGAGENTS_MAX_DEBATE_ROUNDS", 1),
		MaxRiskDiscussRounds: getEnvInt("TRADINGAGENTS_MAX_RISK_ROUNDS", 1),
		MaxRecurLimit:        getEnvInt("TRADINGAGENTS_MAX_RECUR_LIMIT", 100),
		OutputLanguage:       getEnv("TRADINGAGENTS_OUTPUT_LANGUAGE", "English"),
		ResultsDir:           getEnv("TRADINGAGENTS_RESULTS_DIR", ""),
		DataCacheDir:         getEnv("TRADINGAGENTS_CACHE_DIR", ""),
	}

	// Temperature
	if tempStr := os.Getenv("TRADINGAGENTS_TEMPERATURE"); tempStr != "" {
		if t, err := strconv.ParseFloat(tempStr, 64); err == nil {
			cfg.Temperature = t
			cfg.HasTemperature = true
		}
	}

	// Defaults for paths
	home, _ := os.UserHomeDir()
	if cfg.ResultsDir == "" {
		cfg.ResultsDir = home + "/.tradingagents/logs"
	}
	if cfg.DataCacheDir == "" {
		cfg.DataCacheDir = home + "/.tradingagents/cache"
	}

	// Auto-detect LLM provider if not explicitly set
	if cfg.LLMProvider == "google" || cfg.LLMProvider == "gemini" {
		if cfg.GoogleAPIKey == "" {
			log.Println("[WARN] GOOGLE_API_KEY not set — Gemini calls will fail")
		}
	} else if cfg.LLMProvider == "deepseek" {
		if cfg.DeepSeekAPIKey == "" {
			log.Println("[WARN] DEEPSEEK_API_KEY not set — DeepSeek calls will fail")
		}
		// Defaults for DeepSeek models
		if !isEnvSet("TRADINGAGENTS_DEEP_THINK_LLM") {
			cfg.DeepThinkLLM = "deepseek-chat"
		}
		if !isEnvSet("TRADINGAGENTS_QUICK_THINK_LLM") {
			cfg.QuickThinkLLM = "deepseek-chat"
		}
		if cfg.LLMBackendURL == "" {
			cfg.LLMBackendURL = "https://api.deepseek.com/v1"
		}
	} else if cfg.LLMProvider == "openai" {
		if cfg.OpenAIAPIKey == "" {
			log.Println("[WARN] OPENAI_API_KEY not set — OpenAI calls will fail")
		}
		if cfg.LLMBackendURL == "" {
			cfg.LLMBackendURL = "https://api.openai.com/v1"
		}
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func isEnvSet(key string) bool {
	return os.Getenv(key) != ""
}

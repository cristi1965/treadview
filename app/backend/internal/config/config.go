package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	// Server
	Port       string
	AdminToken string

	// LLM Provider
	LLMProvider    string
	DeepThinkLLM   string
	QuickThinkLLM  string
	LLMBackendURL  string
	Temperature    float64
	HasTemperature bool

	// Gemini
	GoogleAPIKey   string
	GeminiProxyURL string
	ThinkingLevel  string

	// DeepSeek
	DeepSeekAPIKey string

	// OpenAI
	OpenAIAPIKey string

	// OpenAI-compatible (for generic endpoints)
	OpenAICompatAPIKey string

	// Data sources
	FREDAPIKey               string
	RunPodAPIKey             string
	ModalTokenID             string
	ModalTokenSecret         string
	LambdaAPIKey             string
	VastAPIKey               string
	GPUCredentialsFile       string
	GPUPriceRefreshInterval  time.Duration
	GPUPriceHistoryRetention time.Duration

	// Analysis
	MaxDebateRounds      int
	MaxRiskDiscussRounds int
	MaxRecurLimit        int
	OutputLanguage       string

	// Paper risk policy
	PaperRiskPolicyVersion  string
	PaperMinStopCoveragePct float64
	PaperMaxDailyLossPct    float64
	PaperMaxDrawdownPct     float64
	PaperMaxStressLossPct   float64

	// Paths
	ResultsDir   string
	DataCacheDir string
	DatabaseDir  string
}

// Load reads configuration from environment and .env files.
func Load() *Config {
	// GPU credentials may come from the real process environment, but never from
	// repository .env files. Preserve the inherited values across godotenv.Load.
	gpuEnvironment := captureGPUCredentialEnvironment()
	alpacaEnvironment := captureAlpacaCredentialEnvironment()
	// Try loading .env from project root (two levels up from backend)
	_ = godotenv.Load("../../.env")
	// Also try local .env
	_ = godotenv.Load(".env")
	restoreGPUCredentialEnvironment(gpuEnvironment)
	restoreAlpacaCredentialEnvironment(alpacaEnvironment)

	cfg := &Config{
		Port:          getEnv("PORT", "8765"),
		AdminToken:    loadAdminToken(),
		LLMProvider:   getEnv("TRADINGAGENTS_LLM_PROVIDER", "google"),
		LLMBackendURL: getEnv("TRADINGAGENTS_LLM_BACKEND_URL", ""),
		DeepThinkLLM:  getEnv("TRADINGAGENTS_DEEP_THINK_LLM", "gemini-3.5-flash"),
		QuickThinkLLM: getEnv("TRADINGAGENTS_QUICK_THINK_LLM", "gemini-3.1-flash-lite"),
		ThinkingLevel: getEnv("TRADINGAGENTS_GOOGLE_THINKING_LEVEL", ""),

		// API Keys
		GoogleAPIKey:       getEnv("GOOGLE_API_KEY", ""),
		GeminiProxyURL:     getEnv("STOCKGOD_GEMINI_PROXY_URL", ""),
		DeepSeekAPIKey:     getEnv("DEEPSEEK_API_KEY", ""),
		OpenAIAPIKey:       getEnv("OPENAI_API_KEY", ""),
		OpenAICompatAPIKey: getEnv("OPENAI_COMPATIBLE_API_KEY", ""),

		FREDAPIKey:               getEnv("FRED_API_KEY", ""),
		RunPodAPIKey:             getEnv("RUNPOD_API_KEY", ""),
		ModalTokenID:             getEnv("MODAL_TOKEN_ID", ""),
		ModalTokenSecret:         getEnv("MODAL_TOKEN_SECRET", ""),
		LambdaAPIKey:             getEnv("LAMBDA_API_KEY", ""),
		VastAPIKey:               getEnv("VAST_API_KEY", ""),
		GPUCredentialsFile:       strings.TrimSpace(os.Getenv("STOCKGOD_GPU_CREDENTIALS_FILE")),
		GPUPriceRefreshInterval:  getEnvDurationRange("STOCKGOD_GPU_REFRESH_INTERVAL", 6*time.Hour, 15*time.Minute, 7*24*time.Hour),
		GPUPriceHistoryRetention: getEnvDurationRange("STOCKGOD_GPU_HISTORY_RETENTION", 90*24*time.Hour, 24*time.Hour, 10*365*24*time.Hour),

		MaxDebateRounds:         getEnvInt("TRADINGAGENTS_MAX_DEBATE_ROUNDS", 1),
		MaxRiskDiscussRounds:    getEnvInt("TRADINGAGENTS_MAX_RISK_ROUNDS", 1),
		MaxRecurLimit:           getEnvInt("TRADINGAGENTS_MAX_RECUR_LIMIT", 100),
		OutputLanguage:          getEnv("TRADINGAGENTS_OUTPUT_LANGUAGE", "Bilingual"),
		PaperRiskPolicyVersion:  getEnv("STOCKGOD_PAPER_RISK_POLICY_VERSION", "paper-risk-v2"),
		PaperMinStopCoveragePct: getEnvFloatRange("STOCKGOD_PAPER_MIN_STOP_COVERAGE_PCT", 100, 0, 100),
		PaperMaxDailyLossPct:    getEnvFloatRange("STOCKGOD_PAPER_MAX_DAILY_LOSS_PCT", 3, 0.01, 100),
		PaperMaxDrawdownPct:     getEnvFloatRange("STOCKGOD_PAPER_MAX_DRAWDOWN_PCT", 12, 0.01, 100),
		PaperMaxStressLossPct:   getEnvFloatRange("STOCKGOD_PAPER_MAX_STRESS_LOSS_PCT", 10, 0.01, 100),
		ResultsDir:              getEnv("TRADINGAGENTS_RESULTS_DIR", ""),
		DataCacheDir:            getEnv("TRADINGAGENTS_CACHE_DIR", ""),
		DatabaseDir:             strings.TrimSpace(os.Getenv("STOCKGOD_DATABASE_DIR")),
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
	if cfg.DatabaseDir == "" {
		cfg.DatabaseDir = defaultDatabaseDir()
	}
	if cfg.GPUCredentialsFile == "" {
		cfg.GPUCredentialsFile = DefaultGPUCredentialsFile()
	}
	if err := ReloadGPUProviderCredentials(cfg); err != nil {
		log.Printf("[WARN] GPU provider credentials unavailable: %v", err)
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
	} else if cfg.LLMProvider == "dual" {
		if cfg.DeepSeekAPIKey == "" || cfg.GoogleAPIKey == "" {
			log.Println("[WARN] dual LLM requires both DEEPSEEK_API_KEY and GOOGLE_API_KEY")
		}
		if !isEnvSet("TRADINGAGENTS_QUICK_THINK_LLM") {
			cfg.QuickThinkLLM = "deepseek-chat"
		}
		if !isEnvSet("TRADINGAGENTS_DEEP_THINK_LLM") {
			cfg.DeepThinkLLM = "gemini-3.1-flash-lite"
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

	NormalizeRuntimeConfig(cfg)
	return cfg
}

func loadAdminToken() string {
	if token := strings.TrimSpace(os.Getenv("STOCKGOD_ADMIN_TOKEN")); token != "" {
		return token
	}
	candidates := []string{strings.TrimSpace(os.Getenv("STOCKGOD_ADMIN_TOKEN_FILE"))}
	if candidates[0] == "" {
		candidates = []string{".stockgod-admin-token", filepath.Join("app", "backend", ".stockgod-admin-token")}
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		raw, err := os.ReadFile(candidate)
		if err == nil && strings.TrimSpace(string(raw)) != "" {
			return strings.TrimSpace(string(raw))
		}
	}
	return ""
}

func defaultDatabaseDir() string {
	if base, err := os.UserConfigDir(); err == nil && strings.TrimSpace(base) != "" {
		return filepath.Join(base, "TradingAgents")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".tradingagents")
	}
	return filepath.Join(os.TempDir(), "TradingAgents")
}

// NormalizeRuntimeConfig keeps provider/model/base-url combinations internally consistent.
func NormalizeRuntimeConfig(cfg *Config) {
	provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	if provider == "" || provider == "gemini" {
		provider = "google"
	}
	cfg.LLMProvider = provider

	switch provider {
	case "dual":
		if cfg.QuickThinkLLM == "" || IsGeminiModel(cfg.QuickThinkLLM) {
			cfg.QuickThinkLLM = "deepseek-chat"
		}
		if !IsGeminiModel(cfg.DeepThinkLLM) {
			cfg.DeepThinkLLM = "gemini-3.1-flash-lite"
		}
		if cfg.LLMBackendURL == "" {
			cfg.LLMBackendURL = "https://api.deepseek.com/v1"
		}
	case "google":
		if !IsGeminiModel(cfg.DeepThinkLLM) {
			cfg.DeepThinkLLM = "gemini-3.5-flash"
		}
		if !IsGeminiModel(cfg.QuickThinkLLM) {
			cfg.QuickThinkLLM = "gemini-3.1-flash-lite"
		}
		cfg.LLMBackendURL = ""
	case "deepseek":
		if cfg.DeepThinkLLM == "" || IsGeminiModel(cfg.DeepThinkLLM) {
			cfg.DeepThinkLLM = "deepseek-chat"
		}
		if cfg.QuickThinkLLM == "" || IsGeminiModel(cfg.QuickThinkLLM) {
			cfg.QuickThinkLLM = "deepseek-chat"
		}
		if cfg.LLMBackendURL == "" {
			cfg.LLMBackendURL = "https://api.deepseek.com/v1"
		}
	case "openai":
		if cfg.LLMBackendURL == "" {
			cfg.LLMBackendURL = "https://api.openai.com/v1"
		}
	}
}

func IsGeminiModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gemini-")
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

func getEnvFloatRange(key string, fallback, minimum, maximum float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		log.Printf("[WARN] %s must be between %.2f and %.2f; using %.2f", key, minimum, maximum, fallback)
		return fallback
	}
	return parsed
}

func getEnvDurationRange(key string, fallback, minimum, maximum time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < minimum || parsed > maximum {
		log.Printf("[WARN] %s must be between %s and %s; using %s", key, minimum, maximum, fallback)
		return fallback
	}
	return parsed
}

func isEnvSet(key string) bool {
	return os.Getenv(key) != ""
}

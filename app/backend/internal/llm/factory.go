package llm

import (
	"fmt"
	"log"

	"trading-agents/internal/config"
)

const (
	ProviderGemini  = "gemini"
	ProviderDeepSeek = "deepseek"
	ProviderOpenAI   = "openai"
	ProviderOpenAICompat = "openai_compatible"
)

// NewClient creates the appropriate LLM client based on config.
func NewClient(cfg *config.Config) (LLMClient, error) {
	var temperature *float32
	if cfg.HasTemperature {
		t := float32(cfg.Temperature)
		temperature = &t
	}

	switch cfg.LLMProvider {
	case ProviderGemini, "google":
		log.Printf("[LLM] Using Gemini provider (model: deep=%s, quick=%s)", cfg.DeepThinkLLM, cfg.QuickThinkLLM)
		return NewGeminiClient(cfg)

	case ProviderDeepSeek:
		apiKey := cfg.DeepSeekAPIKey
		if apiKey == "" {
			return nil, fmt.Errorf("DEEPSEEK_API_KEY not set")
		}
		log.Printf("[LLM] Using DeepSeek provider (model: deep=%s, quick=%s, url=%s)",
			cfg.DeepThinkLLM, cfg.QuickThinkLLM, cfg.LLMBackendURL)
		return NewDeepSeekClient(apiKey, cfg.LLMBackendURL, cfg.DeepThinkLLM, cfg.QuickThinkLLM, temperature), nil

	case ProviderOpenAI:
		apiKey := cfg.OpenAIAPIKey
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		log.Printf("[LLM] Using OpenAI provider (model: deep=%s, quick=%s)", cfg.DeepThinkLLM, cfg.QuickThinkLLM)
		return NewDeepSeekClient(apiKey, cfg.LLMBackendURL, cfg.DeepThinkLLM, cfg.QuickThinkLLM, temperature), nil

	case ProviderOpenAICompat:
		apiKey := cfg.OpenAICompatAPIKey
		log.Printf("[LLM] Using OpenAI-compatible provider (url=%s, model: deep=%s, quick=%s)",
			cfg.LLMBackendURL, cfg.DeepThinkLLM, cfg.QuickThinkLLM)
		return NewDeepSeekClient(apiKey, cfg.LLMBackendURL, cfg.DeepThinkLLM, cfg.QuickThinkLLM, temperature), nil

	default:
		return nil, fmt.Errorf("unknown LLM provider: %s (supported: gemini, deepseek, openai, openai_compatible)", cfg.LLMProvider)
	}
}

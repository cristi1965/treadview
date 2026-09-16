package llm

import (
	"fmt"
	"log"

	"trading-agents/internal/config"
)

const (
	ProviderDual         = "dual"
	ProviderGemini       = "gemini"
	ProviderDeepSeek     = "deepseek"
	ProviderOpenAI       = "openai"
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
	case ProviderDual:
		if cfg.DeepSeekAPIKey == "" || cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("dual provider requires both DEEPSEEK_API_KEY and GOOGLE_API_KEY")
		}
		deepSeek := NewDeepSeekClient(cfg.DeepSeekAPIKey, cfg.LLMBackendURL, cfg.QuickThinkLLM, cfg.QuickThinkLLM, temperature)
		geminiCfg := *cfg
		geminiCfg.LLMProvider = ProviderGemini
		geminiCfg.QuickThinkLLM = cfg.DeepThinkLLM
		gemini, err := NewGeminiClient(&geminiCfg)
		if err != nil {
			return nil, err
		}
		return NewNamedRoutedClient(deepSeek, gemini, "deepseek/"+cfg.QuickThinkLLM, "gemini/"+cfg.DeepThinkLLM), nil
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

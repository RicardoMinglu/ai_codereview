package ai

import (
	"context"
	"fmt"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

type Provider interface {
	Review(ctx context.Context, prompt string) (string, error)
	Name() string
}

func NewProvider(cfg *config.AIConfig) (Provider, error) {
	switch cfg.Provider {
	case "claude":
		return NewClaudeProvider(cfg.Claude)
	case "openai":
		return NewOpenAIProvider(cfg.OpenAI)
	case "gemini":
		return NewGeminiProvider(cfg.Gemini)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}
}

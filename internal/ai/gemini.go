package ai

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

type GeminiProvider struct {
	client *genai.Client
	model  string
}

func NewGeminiProvider(cfg config.GeminiConfig) (*GeminiProvider, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}
	return &GeminiProvider{
		client: client,
		model:  cfg.Model,
	}, nil
}

func (p *GeminiProvider) Review(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.Models.GenerateContent(ctx, p.model, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("gemini API error: %w", err)
	}
	return resp.Text(), nil
}

func (p *GeminiProvider) Name() string {
	return fmt.Sprintf("gemini/%s", p.model)
}

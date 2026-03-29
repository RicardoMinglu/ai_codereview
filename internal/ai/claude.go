package ai

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

type ClaudeProvider struct {
	client *anthropic.Client
	model  string
}

func NewClaudeProvider(cfg config.ClaudeConfig) (*ClaudeProvider, error) {
	client := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	return &ClaudeProvider{
		client: &client,
		model:  cfg.Model,
	}, nil
}

func (p *ClaudeProvider) Review(ctx context.Context, prompt string) (string, error) {
	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude API error: %w", err)
	}

	for _, block := range msg.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("no text response from claude")
}

func (p *ClaudeProvider) Name() string {
	return fmt.Sprintf("claude/%s", p.model)
}

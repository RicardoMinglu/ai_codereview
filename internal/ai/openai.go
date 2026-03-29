package ai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAIProvider(cfg config.OpenAIConfig) (*OpenAIProvider, error) {
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	client := openai.NewClient(opts...)
	return &OpenAIProvider{
		client: &client,
		model:  cfg.Model,
	}, nil
}

func (p *OpenAIProvider) Review(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: p.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}
	return resp.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) Name() string {
	return fmt.Sprintf("openai/%s", p.model)
}

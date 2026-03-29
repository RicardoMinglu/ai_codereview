package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	GitHub  GitHubConfig  `yaml:"github"`
	AI      AIConfig      `yaml:"ai"`
	Review  ReviewConfig  `yaml:"review"`
	Notify  NotifyConfig  `yaml:"notify"`
	Storage StorageConfig `yaml:"storage"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type GitHubConfig struct {
	Token         string `yaml:"token"`
	WebhookSecret string `yaml:"webhook_secret"`
}

type AIConfig struct {
	Provider string       `yaml:"provider"`
	Claude   ClaudeConfig `yaml:"claude"`
	OpenAI   OpenAIConfig `yaml:"openai"`
	Gemini   GeminiConfig `yaml:"gemini"`
}

type ClaudeConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type OpenAIConfig struct {
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"` // 可选，用于中转站/代理，如 https://api.xxx.com/v1
}

type GeminiConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type ReviewConfig struct {
	MaxDiffLines   int      `yaml:"max_diff_lines"`
	Language       string   `yaml:"language"`
	IgnorePatterns []string `yaml:"ignore_patterns"`
}

type NotifyConfig struct {
	DingTalk DingTalkConfig   `yaml:"dingtalk"`
	WeCom    WeComConfig      `yaml:"wecom"`
	Webhooks []WebhookConfig  `yaml:"webhooks"` // 第三方平台机器人（Slack、飞书、自定义等）
}

type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`    // 可选，标识用途
	URL     string `yaml:"url"`     // Webhook URL
	Type    string `yaml:"type"`    // 可选：slack、feishu、custom，默认 custom 发送通用 JSON
}

type DingTalkConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
	Secret     string `yaml:"secret"`
}

type WeComConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
}

type StorageConfig struct {
	Type string `yaml:"type"` // sqlite | mysql | pgsql；留空时 NewStore 按 mysql 处理
	Path string `yaml:"path"` // SQLite 文件路径
	DSN  string `yaml:"dsn"`  // MySQL/PostgreSQL 连接串，如 user:pass@tcp(host:3306)/db
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:    8078,
			BaseURL: "http://localhost:8078",
		},
		AI: AIConfig{
			Provider: "claude",
			Claude: ClaudeConfig{
				Model: "claude-sonnet-4-6",
			},
			OpenAI: OpenAIConfig{
				Model: "gpt-4o",
			},
			Gemini: GeminiConfig{
				Model: "gemini-2.5-flash",
			},
		},
		Review: ReviewConfig{
			MaxDiffLines: 5000,
			Language:     "zh",
			IgnorePatterns: []string{
				"*.lock",
				"vendor/*",
				"node_modules/*",
			},
		},
		Storage: StorageConfig{
			Type: "mysql",
			DSN:  "user:pass@tcp(127.0.0.1:3306)/ai_review?charset=utf8mb4",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.GitHub.Token == "" {
		return fmt.Errorf("github.token is required")
	}
	switch c.AI.Provider {
	case "claude":
		if c.AI.Claude.APIKey == "" {
			return fmt.Errorf("ai.claude.api_key is required when provider is claude")
		}
	case "openai":
		if c.AI.OpenAI.APIKey == "" {
			return fmt.Errorf("ai.openai.api_key is required when provider is openai")
		}
	case "gemini":
		if c.AI.Gemini.APIKey == "" {
			return fmt.Errorf("ai.gemini.api_key is required when provider is gemini")
		}
	default:
		return fmt.Errorf("unsupported ai provider: %s", c.AI.Provider)
	}
	return nil
}

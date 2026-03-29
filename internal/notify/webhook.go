package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/RicardoMinglu/ai_codereview/internal/report"
)

// WebhookNotifier 通用 Webhook 通知，支持 Slack、飞书、自定义等第三方平台
type WebhookNotifier struct {
	url  string
	name string
	typ  string
}

// NewWebhookNotifier 创建通用 Webhook 通知器
func NewWebhookNotifier(url, name, typ string) *WebhookNotifier {
	if typ == "" {
		typ = "custom"
	}
	return &WebhookNotifier{url: url, name: name, typ: typ}
}

// webhookPayload 通用 Webhook 负载格式
type webhookPayload struct {
	Event       string              `json:"event"`
	Repo        string              `json:"repo"`
	Ref         string              `json:"ref"`
	Author      string              `json:"author"`
	CommitSHA   string              `json:"commit_sha"`
	Score       int                 `json:"score"`
	Summary     string              `json:"summary"`
	ReportURL   string              `json:"report_url"`
	Critical    int                 `json:"critical"`
	Warning     int                 `json:"warning"`
	Info        int                 `json:"info"`
	Suggestion  int                 `json:"suggestion"`
	Markdown    string              `json:"markdown,omitempty"` // 便于平台直接渲染
}

func (w *WebhookNotifier) Send(ctx context.Context, r *report.ReviewReport, reportURL string) error {
	counts := r.SeverityCounts()
	scoreEmoji := "🟢"
	if r.Score < 60 {
		scoreEmoji = "🔴"
	} else if r.Score < 80 {
		scoreEmoji = "🟡"
	}

	markdown := fmt.Sprintf(`## %s Code Review

**Repo**: %s | **Ref**: %s | **Author**: %s
**Commit**: %s

%s **Score: %d/100**

**Issues**: 🔴 %d | ⚠️ %d | ℹ️ %d | 💡 %d

**Summary**: %s

[View Report](%s)`,
		scoreEmoji, r.RepoFullName, r.Ref, r.Author, r.CommitSHA[:8],
		scoreEmoji, r.Score,
		counts.Critical, counts.Warning, counts.Info, counts.Suggestion,
		r.Summary, reportURL)

	payload := webhookPayload{
		Event:      "code_review",
		Repo:       r.RepoFullName,
		Ref:        r.Ref,
		Author:     r.Author,
		CommitSHA: r.CommitSHA,
		Score:     r.Score,
		Summary:   r.Summary,
		ReportURL: reportURL,
		Critical:  counts.Critical,
		Warning:   counts.Warning,
		Info:      counts.Info,
		Suggestion: counts.Suggestion,
		Markdown:   markdown,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		name := w.name
		if name == "" {
			name = "webhook"
		}
		return fmt.Errorf("send webhook %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		name := w.name
		if name == "" {
			name = "webhook"
		}
		return fmt.Errorf("webhook %s returned status %d", name, resp.StatusCode)
	}
	return nil
}

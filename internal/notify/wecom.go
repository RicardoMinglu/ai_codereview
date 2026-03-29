package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
)

type WeComNotifier struct {
	webhookURL string
}

func NewWeComNotifier(cfg config.WeComConfig) *WeComNotifier {
	return &WeComNotifier{webhookURL: cfg.WebhookURL}
}

func (w *WeComNotifier) Send(ctx context.Context, r *report.ReviewReport, reportURL string) error {
	scoreEmoji := "🟢"
	if r.Score < 60 {
		scoreEmoji = "🔴"
	} else if r.Score < 80 {
		scoreEmoji = "🟡"
	}

	counts := r.SeverityCounts()
	content := fmt.Sprintf(`## %s Code Review Report

> **Repository**: %s
> **Branch/PR**: %s
> **Author**: %s
> **Commit**: %s

%s **Score: %d/100**

**Issues**: 🔴 Critical: %d | ⚠️ Warning: %d | ℹ️ Info: %d | 💡 Suggestion: %d

**Summary**: %s

[📄 View Full Report](%s)`,
		scoreEmoji, r.RepoFullName, r.Ref, r.Author, r.CommitSHA[:8],
		scoreEmoji, r.Score,
		counts.Critical, counts.Warning, counts.Info, counts.Suggestion,
		r.Summary, reportURL)

	msg := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal wecom message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create wecom request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send wecom message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom returned status %d", resp.StatusCode)
	}
	return nil
}

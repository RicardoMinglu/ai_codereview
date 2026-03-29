package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
)

type DingTalkNotifier struct {
	webhookURL string
	secret     string
}

func NewDingTalkNotifier(cfg config.DingTalkConfig) *DingTalkNotifier {
	return &DingTalkNotifier{
		webhookURL: cfg.WebhookURL,
		secret:     cfg.Secret,
	}
}

func (d *DingTalkNotifier) Send(ctx context.Context, r *report.ReviewReport, reportURL string) error {
	scoreEmoji := "🟢"
	if r.Score < 60 {
		scoreEmoji = "🔴"
	} else if r.Score < 80 {
		scoreEmoji = "🟡"
	}

	counts := r.SeverityCounts()
	statusText := "优秀"
	if r.Score < 60 {
		statusText = "需立即处理"
	} else if r.Score < 80 {
		statusText = "建议优化"
	}

	commitShort := r.CommitSHA
	if len(commitShort) > 8 {
		commitShort = commitShort[:8]
	}

	title := fmt.Sprintf("代码评审｜%s｜%s", r.RepoFullName, r.Ref)
	text := fmt.Sprintf(`## %s 代码评审报告

> **仓库**：%s  
> **分支/PR**：%s  
> **作者**：%s  
> **提交**：%s

---

### %s 评分：**%d / 100**（%s）

**问题分布**

- 🔴 Critical：%d
- ⚠️ Warning：%d
- ℹ️ Info：%d
- 💡 Suggestion：%d

**评审摘要**

%s

---

[📄 查看完整报告](%s)`,
		scoreEmoji, r.RepoFullName, r.Ref, r.Author, commitShort,
		scoreEmoji, r.Score, statusText,
		counts.Critical, counts.Warning, counts.Info, counts.Suggestion,
		r.Summary, reportURL)

	msg := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal dingtalk message: %w", err)
	}

	webhookURL := d.webhookURL
	if d.secret != "" {
		webhookURL = d.signURL(webhookURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create dingtalk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send dingtalk message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}
	return nil
}

func (d *DingTalkNotifier) signURL(webhookURL string) string {
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	stringToSign := fmt.Sprintf("%s\n%s", timestamp, d.secret)

	h := hmac.New(sha256.New, []byte(d.secret))
	h.Write([]byte(stringToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))

	return fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, sign)
}

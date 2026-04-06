package notify

import (
	"context"
	"log"
	"strings"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
)

type Notifier interface {
	Send(ctx context.Context, r *report.ReviewReport, reportURL string) error
}

// MultiNotifier sends notifications to all enabled notifiers.
type MultiNotifier struct {
	notifiers []Notifier
}

func NewMultiNotifier(cfg *config.NotifyConfig) *MultiNotifier {
	var notifiers []Notifier
	if cfg.DingTalk.Enabled {
		if strings.TrimSpace(cfg.DingTalk.WebhookURL) == "" {
			log.Printf("notify: dingtalk.enabled 为 true 但 webhook_url 为空，已跳过（请检查项目 notify_json）")
		} else {
			notifiers = append(notifiers, NewDingTalkNotifier(cfg.DingTalk))
		}
	}
	if cfg.WeCom.Enabled {
		if strings.TrimSpace(cfg.WeCom.WebhookURL) == "" {
			log.Printf("notify: wecom.enabled 为 true 但 webhook_url 为空，已跳过（请检查项目 notify_json）")
		} else {
			notifiers = append(notifiers, NewWeComNotifier(cfg.WeCom))
		}
	}
	for _, wh := range cfg.Webhooks {
		if wh.Enabled && wh.URL != "" {
			notifiers = append(notifiers, NewWebhookNotifier(wh.URL, wh.Name, wh.Type))
		}
	}
	return &MultiNotifier{notifiers: notifiers}
}

func (m *MultiNotifier) Send(ctx context.Context, r *report.ReviewReport, reportURL string) error {
	for _, n := range m.notifiers {
		if err := n.Send(ctx, r, reportURL); err != nil {
			log.Printf("notify error: %v", err)
			// continue to try other notifiers
		}
	}
	return nil
}

package notify

import (
	"context"
	"log"

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
		notifiers = append(notifiers, NewDingTalkNotifier(cfg.DingTalk))
	}
	if cfg.WeCom.Enabled {
		notifiers = append(notifiers, NewWeComNotifier(cfg.WeCom))
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

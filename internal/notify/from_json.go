package notify

import (
	"encoding/json"
	"log"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

// NotifierFromProjectJSON 从 github_projects.notify_json 解析通知配置；为空则无任何渠道。
// JSON 解析失败时打日志并返回空配置。
func NotifierFromProjectJSON(jsonRaw []byte) Notifier {
	empty := &config.NotifyConfig{}
	if len(jsonRaw) == 0 {
		return NewMultiNotifier(empty)
	}
	var n config.NotifyConfig
	if err := json.Unmarshal(jsonRaw, &n); err != nil {
		log.Printf("parse project notify_json: %v", err)
		return NewMultiNotifier(empty)
	}
	return NewMultiNotifier(&n)
}

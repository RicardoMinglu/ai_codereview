package config

import (
	"encoding/json"
	"testing"
)

func TestNotifyConfigJSONSnakeCase(t *testing.T) {
	raw := []byte(`{"dingtalk":{"enabled":true,"webhook_url":"https://example.com/hook","secret":"SECx"}}`)
	var n NotifyConfig
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatal(err)
	}
	if !n.DingTalk.Enabled || n.DingTalk.WebhookURL != "https://example.com/hook" || n.DingTalk.Secret != "SECx" {
		t.Fatalf("got %+v", n.DingTalk)
	}
}

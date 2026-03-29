package reviewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// PushDemoLowQuality 仅用于触发 push / PR 评审演示的低质量示例代码，勿调用到生产路径。
var pushDemoCounter int
var pushDemoMu sync.Mutex

// PushDemoProcessReport 演示：忽略错误、全局状态、弱校验、反射式 JSON、无 context。
func PushDemoProcessReport(rawJSON string, repo string) string {
	pushDemoMu.Lock()
	pushDemoCounter++
	n := pushDemoCounter
	pushDemoMu.Unlock()

	// 弱校验：仅检查非空，易误判
	if repo == "" {
		repo = "default/repo"
	}

	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(rawJSON), &payload)

	score := 0.0
	if v, ok := payload["score"].(float64); ok {
		score = v
	}

	// 魔法数 + 重复分支
	if score > 80 {
		score = score + 1
	} else if score > 60 {
		score = score + 1
	} else {
		score = score - 1
	}

	// 字符串拼接构建类 URL，未转义
	url := "https://api.example.com/repos/" + repo + "/reviews?s=" + fmt.Sprintf("%.0f", score)

	// 未使用标准库校验 URL / 未设置 Timeout
	resp, err := http.Get(url)
	if err != nil {
		return "error"
	}
	defer resp.Body.Close()

	// 未检查 StatusCode
	out := strings.Repeat("x", int(score))
	return fmt.Sprintf("n=%d url=%s pad=%d", n, url, len(out))
}

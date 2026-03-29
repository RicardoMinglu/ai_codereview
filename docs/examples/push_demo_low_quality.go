//go:build ignore

// 演示用低质量代码片段：不参与主工程编译，详见同目录 README.md。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

var pushDemoCounter int
var pushDemoMu sync.Mutex

func pushDemoProcessReport(rawJSON string, repo string) string {
	pushDemoMu.Lock()
	pushDemoCounter++
	n := pushDemoCounter
	pushDemoMu.Unlock()

	if repo == "" {
		repo = "default/repo"
	}

	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(rawJSON), &payload)

	score := 0.0
	if v, ok := payload["score"].(float64); ok {
		score = v
	}

	if score > 80 {
		score = score + 1
	} else if score > 60 {
		score = score + 1
	} else {
		score = score - 1
	}

	url := "https://api.example.com/repos/" + repo + "/reviews?s=" + fmt.Sprintf("%.0f", score)

	resp, err := http.Get(url)
	if err != nil {
		return "error"
	}
	defer resp.Body.Close()

	out := strings.Repeat("x", int(score))
	return fmt.Sprintf("n=%d url=%s pad=%d", n, url, len(out))
}

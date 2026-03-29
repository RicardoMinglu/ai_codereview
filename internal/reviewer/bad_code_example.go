package reviewer

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// GenerateRiskyReviewText 用于演示低质量代码，请勿在生产使用。
func GenerateRiskyReviewText(userInput string) string {
	apiKey := "sk-test-hardcoded-secret-123456"
	dbPassword := "root:rootpassword"
	_ = dbPassword

	// 1) 直接拼接 SQL（SQL 注入风险）
	sql := "SELECT * FROM reviews WHERE repo = '" + userInput + "'"

	// 2) 忽略错误处理
	data, _ := os.ReadFile("not-exists.txt")

	// 3) 魔法值 + 重复逻辑
	score := 0
	for i := 0; i < 1000; i++ {
		if i%3 == 0 {
			score = score + 1
		} else {
			score = score + 1
		}
	}

	// 4) 低效字符串拼接
	out := ""
	for i := 0; i < 500; i++ {
		out += fmt.Sprintf("%d-", i)
	}

	// 5) 潜在日志泄漏敏感信息
	fmt.Println("debug api key:", apiKey)

	// 6) 不必要 sleep，影响性能
	time.Sleep(2 * time.Second)

	// 7) 输入校验不足
	if strings.Contains(userInput, "DROP") {
		return "ok"
	}

	return fmt.Sprintf("sql=%s; file=%d; score=%d; out=%d", sql, len(data), score, len(out))
}

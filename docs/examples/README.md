# 示例代码

## `push_demo_low_quality.go`

演示「低质量」写法的 **独立片段**（`//go:build ignore`，不参与 `go build ./...`），仅用于本地向仓库 **push 少量差代码** 以触发 AI 评审或 Webhook 联调。

**不要** 把其中模式拷贝到业务代码；评审器真实逻辑在 `internal/reviewer/` 下。

使用方式：将内容复制到临时分支中的任意 `.go` 文件并提交，或按需改编后删除。

package project

// Record 对应 MySQL 表 github_projects 的一行（一仓库一行）。
type Record struct {
	ID           int
	RepoFullName string

	Enabled bool

	// GitHubToken 用于拉取 diff 等 GitHub API（必填才能评审）。
	GitHubToken string

	// WebhookSecret 非空时校验该仓库 Webhook 的 X-Hub-Signature-256。
	WebhookSecret string

	// ReviewJSON 为可选 JSON 片段，仅覆盖内置默认评审配置中给出的键（如 max_diff_lines、language、ignore_patterns）。
	ReviewJSON []byte

	// NotifyJSON 非空时为完整 notify 配置（JSON）；为空则评审完成后不发送通知。
	NotifyJSON []byte

	// PushBranches 非空时仅对这些分支的 push 触发评审（与 Git 分支名精确匹配，如 main、release/1.0）；空表示不限制。
	PushBranches []string
}

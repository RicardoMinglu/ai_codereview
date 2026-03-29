# AI Code Review 需求文档

## 1. 项目概述

AI Code Review 是一个基于大语言模型（LLM）的自动化代码评审服务。当 GitHub 仓库发生 push 或 pull_request 事件时，系统自动获取代码变更（diff），调用 AI 进行评审分析，生成结构化报告，并支持通过钉钉、企业微信等渠道通知团队。

---

## 2. 功能需求

### 2.1 核心功能

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 自动代码评审 | 对 GitHub push/PR 的代码变更进行 AI 评审 | P0 |
| 多 AI 提供商支持 | 支持 Claude、OpenAI GPT、Google Gemini 三种模型 | P0 |
| 结构化评审报告 | 输出 0-100 评分、总结、按文件/行的问题列表 | P0 |
| Web 报告查看 | 提供 Web 界面查看历史评审报告 | P0 |
| 通知推送 | 支持钉钉、企业微信推送评审结果 | P1 |

### 2.2 触发方式

- **Push 事件**：当代码推送到分支时触发，评审最新一次 commit 的 diff
- **Pull Request 事件**：当 PR 创建或更新（synchronize）时触发，评审整个 PR 的 diff

### 2.3 评审输出

- **评分**：0-100 整数，表示整体代码质量
- **总结**：文字形式的整体评审摘要
- **问题列表**：每个问题包含：
  - 文件路径
  - 行号（0 表示不适用）
  - 严重程度：`critical`、`warning`、`info`、`suggestion`
  - 问题类别：`bug`、`security`、`performance`、`style`、`best_practice`
  - 问题描述
  - 修复建议

### 2.4 评审维度

1. **Bugs**：逻辑错误、边界情况、空值处理、竞态条件
2. **Security**：SQL 注入、XSS、硬编码密钥、不安全操作
3. **Performance**：N+1 查询、不必要的内存分配、阻塞操作
4. **Code quality**：可读性、命名、重复代码、复杂度
5. **Best practices**：错误处理、日志、测试、文档

---

## 3. 非功能需求

### 3.1 安全

- GitHub Webhook 支持 HMAC-SHA256 签名校验，防止伪造请求
- 配置中的 API Key、Token 等敏感信息需通过配置文件管理，不应硬编码

### 3.2 性能与可靠性

- Webhook 接收后立即返回 200，评审在后台异步执行，避免超时
- Diff 超过 `max_diff_lines`（默认 5000 行）时按文件顺序截断，避免超出 AI 上下文限制
- 支持文件过滤（ignore_patterns），排除 lock 文件、vendor、node_modules 等无关变更

### 3.3 可扩展性

- AI 提供商采用接口抽象，便于新增其他模型
- 通知渠道采用聚合模式，便于新增 Slack、邮件等

### 3.4 兼容性

- 支持 Go 1.25+
- 存储默认 MySQL（可配置 SQLite / PostgreSQL），须可连上对应数据库服务
- 支持 Docker 部署

---

## 4. 约束与依赖

### 4.1 外部依赖

| 依赖 | 说明 |
|------|------|
| GitHub | 必须使用 GitHub 托管代码，需配置 Webhook 和 Personal Access Token |
| AI API | 至少配置一个 AI 提供商的 API Key（Claude/OpenAI/Gemini） |

### 4.2 配置约束

- `github.token`：必填，用于调用 GitHub API 获取 diff
- `ai.<provider>.api_key`：根据所选 provider 必填对应 API Key
- `webhook_secret`：可选，建议生产环境配置以校验 Webhook 签名

### 4.3 已知限制

- 仅支持 GitHub，不支持 GitLab、Gitee 等
- 评审语言（中文/英文）由配置统一指定，不支持按仓库区分
- 大 diff 会被截断，可能无法覆盖全部变更

---

## 5. 数据模型

### 5.1 评审报告（ReviewReport）

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | UUID |
| RepoFullName | string | 仓库全名，如 owner/repo |
| EventType | string | push / pull_request |
| Ref | string | 分支名或 PR 编号 |
| CommitSHA | string | 提交 SHA |
| Author | string | 作者 |
| CommitMsg | string | 提交信息 |
| HTMLURL | string | GitHub 链接 |
| Score | int | 0-100 评分 |
| Summary | string | 总结 |
| Issues | []Issue | 问题列表 |
| FilesNum | int | 评审文件数 |
| LinesNum | int | 变更行数 |
| AIModel | string | 使用的 AI 模型 |
| Duration | float64 | 评审耗时（秒） |

### 5.2 问题（Issue）

| 字段 | 类型 | 说明 |
|------|------|------|
| File | string | 文件路径 |
| Line | int | 行号 |
| Severity | string | critical/warning/info/suggestion |
| Category | string | bug/security/performance/style/best_practice |
| Message | string | 问题描述 |
| Suggest | string | 修复建议 |

---

## 6. 接口与集成

### 6.1 入站

- **GitHub Webhook**：`POST /webhook/github`
  - 事件类型：push、pull_request
  - Content-Type：application/json
  - 可选：X-Hub-Signature-256 签名校验

### 6.2 出站

- **钉钉**：Webhook 机器人
- **企业微信**：Webhook 机器人

### 6.3 HTTP API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | / | 报告列表，支持 ?repo=、?page= 查询 |
| GET | /report/{id} | 报告详情 |
| GET | /health | 健康检查 |

---

## 7. 版本与变更

| 版本 | 说明 |
|------|------|
| 1.0 | 初始版本，支持 Claude/OpenAI/Gemini、钉钉/企业微信、Web 报告 |

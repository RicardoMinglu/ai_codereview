# AI Code Review 实现文档

本文档描述项目的技术实现方式，包括架构、数据流、模块职责和接口定义。

---

## 1. 整体架构

### 1.1 模块划分

| 模块 | 路径 | 职责 |
|------|------|------|
| 入口 | `cmd/server/main.go` | 加载配置、初始化依赖、启动 HTTP 服务 |
| 配置 | `internal/config/` | YAML 配置解析与校验 |
| Webhook | `internal/webhook/` | 接收 GitHub Webhook、解析事件、触发评审 |
| GitHub | `internal/github/` | 调用 GitHub API 获取 diff |
| AI | `internal/ai/` | Claude / OpenAI / Gemini，统一 Provider 接口 |
| 评审 | `internal/reviewer/` | 构建 prompt、调用 AI、解析结果 |
| 存储 | `internal/report/` | SQLite / MySQL / PostgreSQL，统一 Store 接口 |
| 通知 | `internal/notify/` | 钉钉、企业微信、自定义 Webhook |
| Web | `internal/web/` | 路由、HTTP 处理、模板渲染 |

### 1.2 目录结构

```
ai_code_review/
├── cmd/server/main.go              # 程序入口
├── internal/
│   ├── ai/
│   │   ├── provider.go             # Provider 接口 + 工厂
│   │   ├── claude.go
│   │   ├── openai.go
│   │   └── gemini.go
│   ├── config/config.go
│   ├── github/client.go
│   ├── notify/
│   │   ├── notifier.go             # Notifier 接口 + MultiNotifier
│   │   ├── dingtalk.go
│   │   ├── wecom.go
│   │   └── webhook.go              # 通用 Webhook 通知
│   ├── report/
│   │   ├── model.go               # ReviewReport、Issue
│   │   ├── store.go               # Store 接口 + SQLiteStore + scanRows
│   │   ├── store_factory.go       # 根据 type 创建对应 Store
│   │   ├── store_mysql.go
│   │   └── store_pgsql.go
│   ├── reviewer/reviewer.go
│   ├── webhook/handler.go
│   └── web/
│       ├── server.go
│       ├── handler.go
│       └── templates/
│           ├── index.html
│           └── report.html
├── config.example.yaml
├── Makefile
└── docs/
```

### 1.3 依赖关系

```
main.go
  ├── config.Load()
  ├── report.NewStore(&cfg.Storage)
  │     └── 根据 type 返回 SQLiteStore | MySQLStore | PgSQLStore
  ├── ai.NewProvider(&cfg.AI)
  │     └── 根据 provider 返回 Claude | OpenAI | Gemini
  ├── reviewer.New(provider, &cfg.Review)
  ├── notify.NewMultiNotifier(&cfg.Notify)
  │     └── 聚合 DingTalk + WeCom + Webhooks
  └── web.NewServer(cfg, store, rev, notifier)
        ├── webhook.NewHandler()  → Handle POST /webhook/github
        ├── web.NewHandler()      → 报告列表、详情
        └── 路由：POST /webhook/github, GET /, GET /report/{id}, GET /health
```

---

## 2. 核心流程

### 2.1 数据流（Webhook → 评审完成）

```
1. GitHub push / PR 更新
       ↓
2. GitHub 调用 POST /webhook/github
       ↓
3. Handler.Handle()
   - 读取 body
   - 签名校验（可选，webhook_secret）
   - 解析 X-GitHub-Event
   - 立即返回 200 {"status":"accepted"}
   - go processEvent() 异步处理
       ↓
4. 事件分支
   - push     → handlePush()      → GetCommitDiff()
   - pull_request → handlePullRequest() → GetPRDiff()
       ↓
5. reviewer.Review(ctx, req)
   - filterFiles()       → 按 ignore_patterns 过滤
   - truncateDiff()      → 超过 max_diff_lines 则截断
   - buildPrompt()      → 构建评审 prompt
   - provider.Review()  → 调用 AI
   - parseResult()      → 解析 JSON，兜底 score=70
       ↓
6. store.Save(ctx, rpt)
       ↓
7. notifier.Send(ctx, rpt, reportURL)
   - 逐个调用钉钉 / 企业微信 / webhooks，单渠道失败不影响其他
       ↓
8. 完成
```

### 2.2 时序示意

```
GitHub ──POST──► Webhook Handler ──200 OK──► GitHub
                    │
                    └──go──► 解析事件
                              │
                              ▼
                         GitHub Client ──API──► GitHub
                              │
                              ▼
                         Reviewer ──HTTP──► AI 服务
                              │
                              ▼
                         Store.Save()
                              │
                              ▼
                         Notifier.Send() ──HTTP──► 钉钉/企微/Webhook
```

---

## 3. 各模块实现

### 3.1 Webhook 处理

**文件**: `internal/webhook/handler.go`

| 项目 | 说明 |
|------|------|
| 事件类型 | 仅处理 `push`、`pull_request` |
| Push 过滤 | `deleted==true` 跳过；`len(commits)==0` 跳过 |
| PR 过滤 | 仅 `action` 为 `opened`、`synchronize` |
| 签名校验 | HMAC-SHA256，Header `X-Hub-Signature-256: sha256=<hex>` |
| 异步 | 先 200，再 `go processEvent()` |

**事件结构**（节选）:
- Push: `Repository.FullName`、`HeadCommit`、`Ref`、`Commits`
- PR: `PullRequest`、`Repository.FullName`、`action`

---

### 3.2 GitHub API

**文件**: `internal/github/client.go`

| 方法 | 用途 | 实现 |
|------|------|------|
| GetCommitDiff | Push 时获取 diff | `Repositories.GetCommit` → commit.Files |
| GetPRDiff | PR 时获取 diff | `PullRequests.ListFiles`，分页拉全 |
| GetCompareCommits | 两 commit 对比 | `Repositories.CompareCommits`（当前未用） |
| ParseRepoFullName | 解析 owner/repo | `strings.SplitN(fullName, "/", 2)` |

**DiffResult**: `Files`（Filename, Status, Patch, AddLines, DelLines）、`TotalAdd`、`TotalDel`

---

### 3.3 AI 评审

**文件**: `internal/reviewer/reviewer.go`

**Prompt 结构**:
- 系统角色：senior code reviewer
- 上下文：Repo、Ref、Author、CommitMsg
- 输出格式：纯 JSON，`score`、`summary`、`issues[]`
- 评审维度：Bugs、Security、Performance、Code quality、Best practices

**Issue 字段**: file, line, severity, category, message, suggest

**解析逻辑**:
- 支持从 Markdown ```json ... ``` 中提取
- score 限制在 0–100
- 解析失败时：`score=70`，`summary=原始响应`

---

### 3.4 存储

**接口** (`internal/report/`):

```go
type Store interface {
    Save(ctx context.Context, r *ReviewReport) error
    Get(ctx context.Context, id string) (*ReviewReport, error)
    List(ctx context.Context, repo string, page, pageSize int) ([]*ReviewReport, int, error)
}

type StoreCloser interface {
    Store
    Close() error
}
```

**表结构**（reports）:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT/VARCHAR(36) | 主键，UUID |
| created_at | DATETIME/TIMESTAMP | |
| repo_full_name | TEXT/VARCHAR(255) | |
| event_type | TEXT | push / pull_request |
| ref | TEXT | 分支名或 PR 号 |
| commit_sha | TEXT | |
| author | TEXT | |
| commit_msg | TEXT | |
| html_url | TEXT | GitHub 链接 |
| score | INTEGER | 0–100 |
| summary | TEXT | |
| issues | TEXT/JSON/JSONB | JSON 序列化 |
| files_num | INTEGER | |
| lines_num | INTEGER | |
| ai_model | TEXT | |
| duration | REAL/DOUBLE | 秒 |

**工厂** (`store_factory.go`): 根据 `storage.type` 选择 SQLite / MySQL / PgSQL；`type` 为空时默认 MySQL（与 `config.Load` 默认一致）；`sqlite` 时自动创建目录。

---

### 3.5 通知

**接口**:

```go
type Notifier interface {
    Send(ctx context.Context, r *report.ReviewReport, reportURL string) error
}
```

**实现**:
- **DingTalk**: Markdown，支持 secret 签名 URL（timestamp + sign）
- **WeCom**: Markdown 群机器人
- **Webhook**: 通用 JSON，含 event、repo、score、summary、report_url、各 severity 数量、markdown

**MultiNotifier**: 遍历已启用的通知渠道，单渠道失败只打日志，继续其他渠道。

---

### 3.6 Web 界面

**路由**:
- `GET /` → 报告列表，`?repo=` 按仓库，`?page=` 分页
- `GET /report/{id}` → 报告详情
- `GET /health` → 健康检查

**技术**: `embed.FS` 嵌入 `templates/*.html`，模板函数 `scoreColor`、`slice`、`add`、`subtract`。

---

## 4. 接口汇总

### 4.1 AI Provider

```go
type Provider interface {
    Review(ctx context.Context, prompt string) (string, error)
    Name() string
}
```

### 4.2 Report Store

```go
type Store interface {
    Save(ctx context.Context, r *ReviewReport) error
    Get(ctx context.Context, id string) (*ReviewReport, error)
    List(ctx context.Context, repo string, page, pageSize int) ([]*ReviewReport, int, error)
}
```

### 4.3 Notifier

```go
type Notifier interface {
    Send(ctx context.Context, r *report.ReviewReport, reportURL string) error
}
```

### 4.4 数据模型

```go
type ReviewReport struct {
    ID, CreatedAt, RepoFullName, EventType, Ref, CommitSHA
    Author, CommitMsg, HTMLURL
    Score, Summary, Issues, FilesNum, LinesNum, AIModel, Duration
}

type Issue struct {
    File, Severity, Category, Message, Suggest
    Line int
}
```

---

## 5. 配置

### 5.1 配置结构

| 块 | 主要字段 |
|------|------|
| server | port, base_url |
| github | token, webhook_secret |
| ai | provider, claude/api_key+model, openai/api_key+model+base_url, gemini/api_key+model |
| review | max_diff_lines, language, ignore_patterns |
| notify | dingtalk, wecom, webhooks[] |
| storage | type, path, dsn |

### 5.2 校验规则

- `github.token` 必填
- 根据 `ai.provider` 校验对应 `api_key`
- `storage.type=mysql|pgsql` 时 `storage.dsn` 必填

---

## 6. 关键技术点

| 点 | 实现 |
|------|------|
| 防超时 | Webhook 先 200 再异步处理 |
| 防伪造 | webhook_secret + HMAC-SHA256 |
| 大 diff | 按 max_diff_lines 截断，按文件顺序 |
| 无关文件 | ignore_patterns（*.lock、vendor/* 等） |
| AI 兜底 | JSON 解析失败时 score=70，summary=原文 |
| 多通知 | MultiNotifier 聚合，单渠道失败不影响其他 |

# AI Code Review 使用文档

## 1. 前置要求

- **Go**：1.25 或更高版本（若使用源码运行）
- **GitHub**：需要 GitHub 账号和仓库
- **AI API Key**：至少配置一个 AI 提供商的 API Key
  - Claude：在 [Anthropic Console](https://console.anthropic.com/) 获取
  - OpenAI：在 [OpenAI Platform](https://platform.openai.com/) 获取
  - Gemini：在 [Google AI Studio](https://aistudio.google.com/) 获取

---

## 2. 安装

### 2.1 从源码构建

```bash
# 克隆或进入项目目录
cd /path/to/ai_code_review

# 下载依赖
go mod download

# 编译
go build -o ai-code-review ./cmd/server
```

### 2.2 使用 Docker

```bash
# 构建镜像
docker build -t ai-code-review .

# 运行（挂载 config.yaml；DSN 须指向容器可访问的 MySQL；默认端口 8078）
docker run -p 8078:8078 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  ai-code-review
```

---

## 3. 配置

### 3.1 创建配置文件

```bash
cp config.example.yaml config.yaml
```

### 3.2 配置说明

| 配置项 | 必填 | 说明 |
|--------|------|------|
| **server** | | |
| server.port | 否 | HTTP 端口，默认 8078 |
| server.base_url | 否 | 对外访问地址，用于生成报告链接，默认 http://localhost:8078 |
| **github** | | |
| github.token | **是** | GitHub Personal Access Token，需 `repo` 权限 |
| github.webhook_secret | 否 | Webhook 密钥，用于签名校验，建议生产环境配置 |
| **ai** | | |
| ai.provider | 否 | 使用的 AI：`claude`、`openai`、`gemini`，默认 claude |
| ai.claude.api_key | 条件 | provider=claude 时必填 |
| ai.claude.model | 否 | 模型名，默认 claude-sonnet-4-6 |
| ai.openai.api_key | 条件 | provider=openai 时必填 |
| ai.openai.model | 否 | 模型名，默认 gpt-4o |
| ai.openai.base_url | 否 | OpenAI 兼容 API 的 base_url，用于中转站/代理 |
| ai.gemini.api_key | 条件 | provider=gemini 时必填 |
| ai.gemini.model | 否 | 模型名，默认 gemini-2.5-flash |
| **review** | | |
| review.max_diff_lines | 否 | 最大 diff 行数，超出则截断，默认 5000 |
| review.language | 否 | 评审输出语言：`zh`（中文）、`en`（英文），默认 zh |
| review.ignore_patterns | 否 | 忽略的文件模式，如 *.lock、vendor/* |
| **notify** | | |
| notify.dingtalk.enabled | 否 | 是否启用钉钉，默认 false |
| notify.dingtalk.webhook_url | 条件 | 钉钉机器人 Webhook URL |
| notify.dingtalk.secret | 否 | 钉钉加签密钥（若使用加签） |
| notify.wecom.enabled | 否 | 是否启用企业微信，默认 false |
| notify.wecom.webhook_url | 条件 | 企业微信机器人 Webhook URL |
| notify.webhooks | 否 | 第三方平台 Webhook 列表（Slack、飞书、自定义等） |
| **storage** | | |
| storage.type | 否 | 固定为 **`mysql`**；留空时按 mysql |
| storage.dsn | **是** | MySQL DSN，如 `user:pass@tcp(127.0.0.1:3306)/ai_review?charset=utf8mb4` |

### 3.3 最小配置示例

```yaml
github:
  token: "ghp_xxxxxxxxxxxxxxxxxxxx"

ai:
  provider: "claude"
  claude:
    api_key: "sk-ant-xxxxxxxxxxxxxxxxxxxx"
```

### 3.4 使用中转站 / 代理 API

若使用 OpenAI 兼容格式的中转站（如国内 API 代理），将 `provider` 设为 `openai`，并配置 `base_url`：

```yaml
ai:
  provider: "openai"
  openai:
    api_key: "你的中转站API密钥"
    model: "gpt-4o"   # 或中转站支持的模型名，如 gpt-4、gpt-3.5-turbo
    base_url: "https://api.你的中转站.com/v1"   # 必填，中转站接口地址
```

> 注意：中转站须兼容 OpenAI Chat Completions API 格式，大多数第三方代理均支持。

### 3.5 存储（仅 MySQL）

部署前创建库并执行 [`docs/mysql/init.sql`](mysql/init.sql)。示例：

```yaml
storage:
  type: "mysql"
  dsn: "user:pass@tcp(127.0.0.1:3306)/ai_review?charset=utf8mb4"
```

Docker 部署时让应用容器能访问 MySQL（DSN 指向对应主机或服务名），并挂载 `config.yaml`。

### 3.6 第三方平台 Webhook

除钉钉、企业微信外，可配置任意 Webhook URL 推送评审结果（适用于 Slack、飞书等）：

```yaml
notify:
  webhooks:
    - enabled: true
      name: "slack"
      url: "https://hooks.slack.com/services/xxx"
      type: "custom"
```

 payload 为 JSON，包含 `event`、`repo`、`score`、`summary`、`report_url`、`markdown` 等字段。

---

## 4. 运行

### 4.1 启动服务

```bash
# 使用默认 config.yaml
./ai-code-review

# 指定配置文件
./ai-code-review -config /path/to/config.yaml
```

### 4.2 验证运行

```bash
curl http://localhost:8078/health
```

---

## 5. 配置 GitHub Webhook

### 5.1 创建 Webhook

1. 打开 GitHub 仓库 → **Settings** → **Webhooks** → **Add webhook**
2. 填写：
   - **Payload URL**：`https://<你的域名或IP>:8078/webhook/github`
   - **Content type**：`application/json`
   - **Secret**：与 `config.yaml` 中 `github.webhook_secret` 一致（可选但推荐）
   - **Which events**：选择 **Just the push event** 和 **Pull requests**

### 5.2 本地调试

若在本地运行，可使用 [ngrok](https://ngrok.com/) 等工具暴露端口：

```bash
ngrok http 8078
# 将生成的 https 地址填入 Webhook Payload URL
```

---

## 6. 使用 Web 界面

### 6.1 报告列表

访问 `http://localhost:8078` 查看所有评审报告。

- 支持按仓库筛选：`?repo=owner/repo`
- 支持分页：`?page=2`

### 6.2 报告详情

点击报告或访问 `http://localhost:8078/report/{id}` 查看：

- 评分、总结、耗时
- 按严重程度分类的问题列表
- 每个问题的文件、行号、描述、修复建议

---

## 7. 通知配置

### 7.1 钉钉

1. 在钉钉群中添加「自定义机器人」
2. 选择「加签」安全设置（推荐），记录 Webhook URL 和 Secret
3. 在 `config.yaml` 中配置：

```yaml
notify:
  dingtalk:
    enabled: true
    webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
    secret: "SECxxxxxxxx"
```

### 7.2 企业微信

1. 在企业微信群中添加「群机器人」
2. 获取 Webhook URL
3. 在 `config.yaml` 中配置：

```yaml
notify:
  wecom:
    enabled: true
    webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

---

## 8. 常见问题

### Q: 评审没有触发？

- 检查 Webhook 是否配置正确，Payload URL 是否可访问
- 查看服务日志，确认是否收到 push/pull_request 事件
- 若配置了 webhook_secret，确认与 GitHub 中一致

### Q: 报告链接打不开？

- 确保 `server.base_url` 配置为实际可访问的地址（如公网域名）
- 本地调试时 base_url 可设为 ngrok 地址

### Q: Diff 太大被截断？

- 调整 `review.max_diff_lines`（注意 AI 上下文限制）
- 通过 `review.ignore_patterns` 排除无关文件

### Q: 如何切换 AI 模型？

修改 `config.yaml` 中 `ai.provider` 和对应 provider 的 `api_key`，重启服务即可。

### Q: 数据存储在哪里？

数据仅在 **MySQL**：报告与 `github_projects` 同库；须先建库并执行 [`docs/mysql/init.sql`](mysql/init.sql)。Docker 下挂载 `config.yaml`，DSN 指向可访问的 MySQL。

---

## 9. 生产部署建议

1. **反向代理**：使用 Nginx 等做 HTTPS 和负载均衡
2. **进程管理**：使用 systemd 或 supervisor 管理进程
3. **日志**：将 stdout/stderr 输出到日志文件或集中日志服务
4. **监控**：定期检查 `/health` 端点
5. **安全**：务必配置 `webhook_secret`，避免 API Key 泄露

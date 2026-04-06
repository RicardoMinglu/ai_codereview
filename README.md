# AI Code Review

[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Build Status](https://img.shields.io/github/actions/workflow/status/RicardoMinglu/ai_codereview/.github%2Fworkflows%2Fci.yml?branch=master&label=build)](https://github.com/RicardoMinglu/ai_codereview/actions)
[![License](https://img.shields.io/github/license/ricardominglu/ai_codereview)](LICENSE)
[![Stars](https://img.shields.io/github/stars/ricardominglu/ai_codereview?style=social)](https://github.com/RicardoMinglu/ai_codereview/stargazers)

基于大语言模型的自动化代码评审服务。  
当 GitHub 仓库发生 **`push`（仅分支，不含 tag）** 或 **`pull_request`** 事件时，系统拉取代码变更、调用 AI 评审，并生成结构化报告与可选通知。

## 配置架构（按仓库表驱动）

- **`config.yaml` 只负责**：`server`（端口、对外 `base_url`）、**`storage.dsn`（MySQL）**、`ai`（模型与 API Key）。**不**在本文件里配置 GitHub Token、Webhook、评审或通知。
- **仓库与凭据**：自设计起即为**多仓库**。每个 `owner/repo` 在 MySQL 表 **`github_projects`** 中占一行（推荐 **`/admin/projects`** Web 管理页），至少配置 **GitHub Token**；可选 Webhook Secret、`push_branches`、`review_json`、`notify_json` 等。
- **存储**：**仅 MySQL**；`storage.type` 非 `mysql`（或未配 `dsn`）时无法启动。
- **Webhook**：只处理**已在表中登记且启用**的仓库；未登记仓库的事件会被忽略。

## 功能特性

- **分支 `push`** 与 **`pull_request`**（`opened` / `synchronize`）
- **不评审 tag push**（`refs/tags/*`）
- **Push 分支白名单**：列 **`push_branches`**（管理页「监听 Push 的分支」）；非空则仅这些分支触发 push 评审；**PR 不受此字段限制**
- 结构化结果：0–100 分、问题列表、改进建议
- 模型：Claude、OpenAI、Gemini；OpenAI 兼容中转（`base_url`）
- 通知：钉钉 / 企业微信 / 自定义 Webhook 等在 **`notify_json`** 中配置（JSON 键与代码一致，如 `webhook_url`）
- Web UI：列表 / 详情 / **`/admin/projects`**；**`/setup`** 重定向到项目配置页

## 界面预览

截图目录：[`docs/images/`](docs/images/)。

| 说明 | 文件 |
|------|------|
| 评审列表 | `docs/images/review-list.png` |
| 评审详情 | `docs/images/review-detail.png` |
| 钉钉通知 | `docs/images/dingtalk.png` |

![评审列表](docs/images/review-list.png)

![评审详情](docs/images/review-detail.png)

![钉钉通知](docs/images/dingtalk.png)

## 项目结构（节选）

```text
├── cmd/server/
├── internal/
│   ├── webhook/      # GitHub 事件
│   ├── ai/
│   ├── project/      # 分支策略、表配置模型
│   ├── report/       # MySQL：reports + github_projects
│   └── web/
├── docs/mysql/
│   ├── init.sql                    # 建库脚本（部署前执行）
│   └── alter_push_branches.sql     # 老库补列（按需）
├── config.example.yaml
└── Makefile
```

## 快速开始

### 1) 克隆

```bash
git clone https://github.com/RicardoMinglu/ai_codereview.git
cd ai_codereview
```

### 2) `config.yaml`

```bash
cp config.example.yaml config.yaml
```

填写 **`server`**、**`storage.dsn`**、**`ai`**。其中 **`storage.type` 固定为 `mysql`**。

### 3) MySQL 表

```bash
mysql -h HOST -u USER -p YOUR_DB < docs/mysql/init.sql
```

已有库缺 `push_branches` 时：

```bash
mysql ... < docs/mysql/alter_push_branches.sql
```

### 4) 启动

```bash
make run
```

日志会打印评审列表、项目配置、Webhook 等 URL。若 `github_projects` 为空会有初始化提示。

### 5) 登记仓库

打开 **`http://<host>:<port>/admin/projects`**（或 **`/setup`**），为每个 `owner/repo` 配置至少 **GitHub Token**；按需填写 Webhook Secret、Push 分支、**`review_json`** / **`notify_json`**。

### 6) GitHub Webhook

- URL：`https://<公网或 ngrok>/webhook/github`
- 事件：**Pushes**、**Pull requests**，`application/json`

**`server.base_url`** 请与报告链接、钉钉等通知中的域名一致（常为 ngrok 或正式域名）。

## `config.yaml` 示例

```yaml
server:
  port: 8078
  base_url: "http://localhost:8078"

ai:
  provider: "openai"
  openai:
    api_key: "sk-xxx"
    model: "gpt-4o"

storage:
  type: "mysql"
  dsn: "user:pass@tcp(127.0.0.1:3306)/ai_review?charset=utf8mb4"
```

OpenAI 兼容中转：

```yaml
ai:
  provider: "openai"
  openai:
    api_key: "proxy-key"
    model: "gpt-4o"
    base_url: "https://api.example.com/v1"
```

## 通知 `notify_json` 要点

- 使用 **JSON**，键名与程序一致（如 `dingtalk.webhook_url`，勿依赖仅 YAML 的字段名）。
- 钉钉仅关键词、未开「加签」时 **`secret` 留空**；开加签则填 **`SEC…`**。

## 常用命令

| 命令 | 说明 |
|------|------|
| `make build` / `make run` | 编译 / 运行 |
| `make test` | 测试 |
| `make docker-build` / `make docker-run` | 镜像 / 容器（需自备 MySQL 与 `config.yaml`） |
| `make init-config` | 生成 `config.yaml` |
| `make help` | 帮助 |

## 访问地址（端口以配置为准）

- `/` 评审列表，`/report/{id}` 详情，`/admin/projects` 项目表， `/setup` 跳转管理，`POST /webhook/github`，`/health`

## 文档与协作

- [使用文档](使用文档.md)、[docs/USAGE.md](docs/USAGE.md)（若与当前行为不一致，以本 README 与代码为准）
- [CONTRIBUTING.md](CONTRIBUTING.md)、[SECURITY.md](SECURITY.md)

默认分支 **`master`**，PR 请指向该分支。

## 运行依赖

- Go 1.25+
- **MySQL** + 已执行 **`docs/mysql/init.sql`**
- 表内各仓库 **GitHub Token**（`repo`）与 **`config.yaml` 内 AI Key**

## 安全说明

管理页与报告页默认**无登录**；Webhook 验签依赖表内 **`webhook_secret`** 与 GitHub 一致。生产环境请 HTTPS、鉴权、限流；密钥勿入库。详见 **[SECURITY.md](SECURITY.md)**。

## License

MIT

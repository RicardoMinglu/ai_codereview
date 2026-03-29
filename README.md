# AI Code Review

[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Build Status](https://img.shields.io/github/actions/workflow/status/RicardoMinglu/ai_codereview/ci.yml?branch=master&label=build)](https://github.com/RicardoMinglu/ai_codereview/actions)
[![License](https://img.shields.io/github/license/RicardoMinglu/ai_codereview)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/RicardoMinglu/ai_codereview?style=social)](https://github.com/RicardoMinglu/ai_codereview/stargazers)

基于大语言模型的自动化代码评审服务。  
当 GitHub 仓库发生 `push` 或 `pull_request` 事件时，系统会自动获取代码变更、调用 AI 进行评审，并生成结构化报告与通知。

适合团队在 CI/CD 之外补充“语义层”的代码质量反馈，也适合个人项目做持续质量守护。

## 功能特性

- 自动评审：监听 GitHub Webhook，按 `push` / `PR` 触发评审
- 结构化结果：生成 0-100 分、问题列表、改进建议
- 多模型支持：Claude、OpenAI、Gemini，支持 OpenAI 兼容 API
- 多存储支持：SQLite、MySQL、PostgreSQL
- 多通知渠道：钉钉、企业微信、自定义 Webhook（Slack、飞书等）
- 可视化报告：Web 页面查看评审列表、详情、仓库筛选

## 界面预览

截图放在仓库内目录 [`docs/images/`](docs/images/)，文件名与下方引用一致即可（也可改成 `.jpg`，同步改 README 里后缀）。

| 说明 | 文件 |
|------|------|
| 评审列表 | `docs/images/review-list.png` |
| 评审详情 | `docs/images/review-detail.png` |
| 钉钉通知 | `docs/images/dingtalk.png` |

![评审列表](docs/images/review-list.png)

![评审详情](docs/images/review-detail.png)

![钉钉通知](docs/images/dingtalk.png)

## 项目结构

```text
.
├── cmd/                    # 程序入口
├── internal/
│   ├── webhook/            # GitHub 事件处理
│   ├── ai/                 # 大模型调用与提示词
│   ├── report/             # 评审结果存储与查询
│   └── web/                # Web 页面与 HTTP 接口
├── docs/                   # 详细文档
│   ├── images/             # README 用界面截图（见「界面预览」）
│   └── mysql/init.sql      # 可选 MySQL 表结构（服务也会自动迁移）
├── config.example.yaml     # 配置示例
└── Makefile                # 常用开发命令
```

## 快速开始

### 1) 克隆仓库

```bash
git clone https://github.com/RicardoMinglu/ai_codereview.git
cd ai_codereview
```

Go 模块路径：`github.com/RicardoMinglu/ai_codereview`（与仓库一致，便于 `go install` / 依赖引用）。

### 2) 初始化配置

```bash
cp config.example.yaml config.yaml
```

编辑 `config.yaml`，至少填写：
- `github.token`
- `ai.provider`
- 对应模型供应商的 `api_key`

### 3) 启动服务

```bash
make run
# 或
make build && ./ai-code-review -config config.yaml
```

### 4) 配置 GitHub Webhook

- URL：`https://<your-domain>/webhook/github`
- 事件：`Pushes`、`Pull requests`
- Content type：`application/json`

本地调试建议使用 [ngrok](https://ngrok.com/) 暴露本地地址。

## 配置示例

最小可用配置：

```yaml
github:
  token: "ghp_xxx"

ai:
  provider: "openai" # claude | openai | gemini
  openai:
    api_key: "sk-xxx"
    model: "gpt-4o"
```

### 数据库

| 类型 | 配置 |
|------|------|
| MySQL（程序与 `config.example.yaml` 默认） | `storage.type: mysql`，`storage.dsn: user:pass@tcp(host:3306)/db` |
| SQLite | `storage.type: sqlite`，`storage.path: ./data/reviews.db` |
| PostgreSQL | `storage.type: pgsql`，`storage.dsn: postgres://user:pass@host:5432/db` |

### 通知

- 钉钉：`notify.dingtalk.enabled: true`，并填写 `webhook_url`、`secret`
- 企业微信：`notify.wecom.enabled: true`，并填写 `webhook_url`
- 第三方 Webhook：配置 `notify.webhooks` 列表

### OpenAI 兼容 API（中转站）

```yaml
ai:
  provider: "openai"
  openai:
    api_key: "proxy-key"
    model: "gpt-4o"
    base_url: "https://api.example.com/v1"
```

## 常用命令

| 命令 | 说明 |
|------|------|
| `make build` | 编译 |
| `make run` | 编译并运行 |
| `make test` | 运行测试 |
| `make docker-build` | 构建 Docker 镜像 |
| `make docker-run` | 运行 Docker 容器（映射 `8078`，与默认 `server.port` 一致） |
| `make init-config` | 从示例创建 `config.yaml` |
| `make help` | 显示帮助 |

## 访问地址

- 报告列表：`http://localhost:8078/`
- 报告详情：`http://localhost:8078/report/{id}`
- 健康检查：`http://localhost:8078/health`

## 文档

- [使用文档](docs/USAGE.md)：详细配置与使用说明
- [实现文档](docs/IMPLEMENTATION.md)：架构、数据流、模块实现
- [GitHub 接入指南](docs/GITHUB_INTEGRATION.md)：Webhook 与权限配置
- [需求文档](docs/REQUIREMENTS.md)：功能与非功能需求

## 运行依赖

- Go 1.25+
- GitHub Personal Access Token（`repo` 权限）
- 至少一个 AI API Key（Claude / OpenAI / Gemini）

## 开源协作

默认分支为 **`master`**（与历史较长的 `main` 无继承关系时，可在 GitHub 设置中将默认分支改为 `master`，并按需删除远端 `main`）。

欢迎提交 Issue 和 Pull Request。建议流程：

1. Fork 本仓库并基于 **`master`** 创建功能分支
2. 提交改动并补充必要测试
3. 向 **`master`** 发起 PR，描述动机、改动点和验证方式

如果是较大改动，建议先开 Issue 讨论设计方向。

## 安全说明

- 不要将真实密钥提交到仓库（`config.yaml`、含密钥的本地配置文件勿入库）
- 建议使用环境隔离或密钥管理服务托管令牌
- 生产环境务必配置 **`github.webhook_secret`**，校验 Webhook 签名
- 报告 **Web 界面无登录**：勿对公网裸奔；建议仅内网访问，或通过反向代理做 **HTTPS + 鉴权**（如 Basic、IP 允许列表等）

## License

MIT

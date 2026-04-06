# 仓库（项目）配置管理界面说明

## 功能概述

通过 Web 界面维护 MySQL 表 **`github_projects`**，无需手写 SQL。每个 `owner/repo` 对应一行。

## 访问地址

```
http://localhost:8078/admin/projects
```

## 功能特性

### 1. 列表
- 查看已登记仓库及启用状态
- 配置概览（Token / 评审 / 通知等是否已填）

### 2. 添加项目
点击「添加项目」后：

- **仓库全名**（必填）：`owner/repo`，例如 `microsoft/vscode`
- **启用**：勾选表示接收该仓库的 Webhook 并评审
- **GitHub Token**（必填）：该行仓库用于 API 的 Token（`repo`）
- **Webhook Secret**（可选）：与 GitHub Webhook 一致时启用签名校验；留空则不验签
- **监听 Push 的分支**（可选）：非空则仅这些分支的 push 触发；PR 不受限
- **评审配置**（可选）：JSON 片段，与内置默认评审配置合并
- **通知配置**（可选）：JSON 完整通知结构；留空则评审完成后不通知

### 3. 编辑 / 删除
与添加字段相同；删除即移除该仓库一行。

### 4. JSON 校验
编辑评审 / 通知 JSON 时会做格式校验与提示。

## 行为说明

- **仅处理已登记且启用的仓库**；其它 Webhook 事件忽略。
- **`config.yaml`** 只提供 `server`、`storage`、`ai`；不包含各仓库存 Token 或通知。

## 配置示例

### 示例 1：最简（Token + 默认评审，不通知）

```
仓库全名: org/repo-a
启用: ✓
GitHub Token: ghp_xxxxxxxxxxxxxxxxxxxx
Webhook Secret: whsec_xxx （Production 建议填写）
监听 Push 的分支: （留空表示不限制）
评审配置: （留空则用默认）
通知配置: （留空则不通知）
```

### 示例 2：评审 + 钉钉

```
仓库全名: org/repo-b
启用: ✓
GitHub Token: ghp_xxxxxxxxxxxxxxxxxxxx
Webhook Secret: whsec_xxxxxxxxxxxxxxxxxxxx
评审配置:
{
  "max_diff_lines": 8000,
  "language": "en",
  "ignore_patterns": ["*.test.js", "*.spec.ts"]
}
通知配置:
{
  "dingtalk": {
    "enabled": true,
    "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
    "secret": "SECxxxxxxxx"
  }
}
```

### 示例 3：禁用

取消勾选「启用」或 `enabled=0`，该仓库不再评审。

## 注意事项

1. 修改后立即对新事件生效，一般无需重启。
2. `review_json` / `notify_json` 须为合法 JSON。
3. 本服务运行依赖 **MySQL** 与已执行的 `docs/mysql/init.sql`。

## 配置字段说明

### 评审配置 (review_json)

可与默认合并的字段示例：

```json
{
  "max_diff_lines": 5000,
  "language": "zh",
  "ignore_patterns": [
    "*.lock",
    "*.sum",
    "vendor/*",
    "node_modules/*"
  ]
}
```

### 通知配置 (notify_json)

完整通知结构示例：

```json
{
  "dingtalk": {
    "enabled": true,
    "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
    "secret": "SECxxxxxxxx"
  },
  "wecom": {
    "enabled": false
  },
  "webhooks": [
    {
      "enabled": true,
      "name": "slack",
      "url": "https://hooks.slack.com/services/xxx",
      "type": "custom"
    }
  ]
}
```

## 常见问题

### Q: 看不到管理页或报错？

核对 `storage.type` 为 `mysql`、DSN 正确且已执行 `docs/mysql/init.sql`。

### Q: 修改后要重启吗？

通常不需要；若改了 `config.yaml` 中的 `server`/`ai` 等则需重启。

### Q: JSON 报错？

按界面提示修正，或用在线 JSON 校验工具检查。

### Q: 可以同时开多个通知渠道吗？

可以，在 `notify_json` 中按需启用多个渠道。

## 界面预览

列表、编辑表单以实际部署页面为准；截图可参考 `docs/images/`。

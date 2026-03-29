# GitHub 接入指南

本文档手把手教你如何将 AI Code Review 服务接入 GitHub 仓库，实现 push 和 Pull Request 的自动代码评审。

---

## 一、前置准备

在开始前，请确保：

1. **AI Code Review 服务已启动**，可访问 `http://<你的服务地址>:8078/health` 并返回 `ok`
2. **你拥有目标 GitHub 仓库的管理员权限**（用于配置 Webhook）
3. **服务地址对 GitHub 可访问**：若在本地运行，需使用 ngrok 等工具暴露到公网

---

## 二、创建 GitHub Personal Access Token

Token 用于调用 GitHub API 获取代码 diff，**必须**创建。

### 步骤

1. 登录 GitHub，点击右上角头像 → **Settings**
2. 左侧菜单最下方 → **Developer settings**
3. 左侧 → **Personal access tokens** → **Tokens (classic)**
4. 点击 **Generate new token** → **Generate new token (classic)**
5. 填写：
   - **Note**：如 `AI Code Review`
   - **Expiration**：选择有效期（建议 90 天或 No expiration）
   - **Select scopes**：勾选 **repo**（包含所有子权限）
6. 点击 **Generate token**
7. **复制生成的 token**（形如 `ghp_xxxxxxxxxxxx`），离开页面后无法再次查看

### 权限说明

| 权限 | 用途 |
|------|------|
| repo | 访问仓库内容，获取 commit 和 PR 的 diff |

---

## 三、配置 config.yaml

将 Token 填入配置文件：

```yaml
github:
  token: "ghp_你刚才复制的Token"
  webhook_secret: ""   # 先留空，第四步创建 Webhook 时再填
```

保存后重启服务。

---

## 四、配置 GitHub Webhook

### 4.1 进入 Webhook 设置页

1. 打开你的 **GitHub 仓库**
2. 点击 **Settings**
3. 左侧菜单 → **Webhooks**
4. 点击 **Add webhook**

### 4.2 填写 Webhook 参数

| 字段 | 填写说明 |
|------|----------|
| **Payload URL** | `https://<你的服务地址>/webhook/github`<br>例如：`https://your-domain.com/webhook/github`<br>本地调试：`https://xxxx.ngrok.io/webhook/github` |
| **Content type** | 选择 **application/json** |
| **Secret** | 可选。若填写，需与 `config.yaml` 中 `github.webhook_secret` 一致，用于签名校验 |
| **SSL verification** | 勾选 **Enable SSL verification**（推荐） |
| **Which events would you like to trigger this webhook?** | 选择 **Let me select individual events** |

### 4.3 选择要触发的事件

勾选以下两项：

- **Push events**：代码推送到分支时触发
- **Pull requests**：创建或更新 PR 时触发

然后点击 **Add webhook**。

### 4.4 若配置了 Secret

在 `config.yaml` 中填入与 GitHub 相同的 Secret：

```yaml
github:
  token: "ghp_xxx"
  webhook_secret: "你在GitHub填的Secret"
```

重启服务后，Webhook 校验才会生效。

---

## 五、本地调试（服务在本地时）

GitHub 只能向公网 URL 发送 Webhook，本地 `localhost` 无法直接接收。

### 使用 ngrok

1. 安装 [ngrok](https://ngrok.com/download)
2. 启动 AI Code Review 服务（如 `./ai-code-review`）
3. 新开终端执行：

   ```bash
   ngrok http 8078
   ```

4. 复制 ngrok 显示的 Forwarding 地址，例如 `https://a1b2c3d4.ngrok-free.app`
5. 在 GitHub Webhook 的 **Payload URL** 中填写：

   ```
   https://a1b2c3d4.ngrok-free.app/webhook/github
   ```

6. 在 `config.yaml` 中设置 `base_url`，方便报告链接正确：

   ```yaml
   server:
     base_url: "https://a1b2c3d4.ngrok-free.app"
   ```

### 注意

- 免费版 ngrok 每次重启地址会变，需同步更新 GitHub Webhook 和 `base_url`
- 若使用付费固定域名，可保持不变

---

## 六、验证接入

### 6.1 检查 Webhook 状态

1. 在 GitHub 仓库 → **Settings** → **Webhooks**
2. 点击你刚创建的 Webhook
3. 查看 **Recent Deliveries**：
   - 绿色勾：最近一次发送成功
   - 红色叉：失败，可点开查看响应和错误信息

### 6.2 触发一次评审

**方式一：Push**

```bash
git add .
git commit -m "test: trigger code review"
git push origin master
```

**方式二：Pull Request**

1. 新建分支并 push
2. 在 GitHub 上创建 Pull Request

### 6.3 查看结果

1. 打开 `http://<你的服务地址>:8078`
2. 应能看到刚触发的评审报告
3. 点击报告可查看详情（评分、问题列表等）

---

## 七、常见问题

### 1. Webhook 显示红色叉，请求失败

**可能原因：**

- Payload URL 无法访问：检查服务是否启动、端口是否正确、防火墙是否放行
- 本地服务：必须用 ngrok 等工具暴露到公网

**排查：**

- 在浏览器访问 `https://你的地址/webhook/github`，应返回 405（GET 不允许），说明服务可达
- 查看服务日志，确认是否收到请求

### 2. Webhook 显示 401 Unauthorized

说明启用了 **Secret**，但签名校验失败：

- 确认 `config.yaml` 中 `github.webhook_secret` 与 GitHub Webhook 中填写的 **Secret** 完全一致（含空格、大小写）

### 3. 评审没有触发

- 确认勾选了 **Push events** 和 **Pull requests**
- 检查服务日志：`processing push to xxx` 或 `processing PR #x on xxx` 表示已收到事件
- 若配置了 `ignore_patterns`，可能所有变更都被过滤，导致空 diff 不调用 AI

### 4. 报告链接打不开

- 确保 `server.base_url` 为实际可访问的地址
- 本地调试时 base_url 应设为 ngrok 的 https 地址

### 5. 获取 diff 失败（get commit diff error / get PR diff error）

- 检查 Token 是否有效、是否过期
- 确认 Token 有 **repo** 权限
- 确认仓库对 Token 所属账号可见（私有仓库需有访问权限）

---

## 八、接入流程图

```
┌─────────────────┐     Push / PR     ┌──────────────────┐
│  GitHub 仓库     │ ────────────────► │  GitHub Webhook   │
└─────────────────┘                   └────────┬─────────┘
                                               │
                                               │ HTTP POST
                                               ▼
┌─────────────────┐     Webhook      ┌──────────────────┐
│  AI Code Review  │ ◄────────────────│  /webhook/github  │
│  服务 (8078)     │                   └──────────────────┘
└────────┬────────┘
         │
         │ 1. 校验签名
         │ 2. 调用 GitHub API 获取 diff
         │ 3. 调用 AI 评审
         │ 4. 存储报告
         │ 5. 发送通知（若配置）
         ▼
┌─────────────────┐
│  Web 报告列表     │  http://xxx:8078/
└─────────────────┘
```

---

## 九、快速检查清单

接入前可逐项确认：

- [ ] 已创建 GitHub Personal Access Token（repo 权限）
- [ ] 已把 Token 填入 `config.yaml` 的 `github.token`
- [ ] 服务已启动，`/health` 返回 ok
- [ ] 已在 GitHub 仓库添加 Webhook

Webhook 配置：

- [ ] Payload URL 为 `https://<域名或IP>/webhook/github`
- [ ] Content type 为 `application/json`
- [ ] 已勾选 Push events 和 Pull requests
- [ ] 若配置了 Secret，`config.yaml` 中 `webhook_secret` 与之一致

本地调试时：

- [ ] 已用 ngrok 暴露 8078 端口
- [ ] 已将 ngrok 地址填入 Webhook Payload URL
- [ ] 已将 ngrok 地址填入 `server.base_url`

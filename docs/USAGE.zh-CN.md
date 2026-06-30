# Ops-Agent 使用文档

面向第一次使用或需要交接给他人的快速上手指南。实现进度以当前代码为准（M2.5：Webhook + 待办；M3 AI 回复尚未接入）。

---

## 1. 这是什么

Ops-Agent 是一个**终端 TUI 程序**，用来：

- 通过 **GitHub Webhook** 把 Issue/PR 评论事件推送到本机
- 在左侧**待办列表**里集中展示需要处理的 Issue
- 用 `gh` 查看 Issue 详情（按 `i`）
- 后续版本将接入本地 AI 自动分析与回复（M3）

**你需要准备：**

| 依赖 | 用途 |
|------|------|
| [GitHub CLI (`gh`)](https://cli.github.com/) | 登录 GitHub、查看 Issue |
| Go 1.22+（自行编译时） | 构建二进制 |
| [smee.io](https://smee.io/) 频道 | 把 GitHub Webhook 转到本机（已内嵌，无需单独跑 smee-client） |
| llama-server（可选） | M3 前的 AI 功能仅占位 |

---

## 2. 安装与启动

### 2.1 编译

```bash
git clone <你的仓库地址>
cd Ops-Agent
go build -o ops-agent ./cmd/ops-agent
```

或使用 Makefile：

```bash
make build
```

### 2.2 登录 GitHub

```bash
gh auth login
gh auth status   # 应显示已登录
```

### 2.3 配置文件

复制示例配置到项目根目录：

```bash
cp .ops-agent.yaml.example .ops-agent.yaml
```

配置加载顺序：

1. 环境变量 `OPS_AGENT_CONFIG` 指定路径
2. 当前目录 `.ops-agent.yaml`
3. `~/.config/ops-agent/config.yaml`

待办数据保存在：`~/.local/share/ops-agent/todo.json`

### 2.4 启动 TUI

```bash
./ops-agent
```

启动后会看到：

- 顶部：Banner、状态栏（模型 / 模式 / 当前 cwd 仓库 / 待办数量）
- 左侧：**待办**列表
- 右侧：输出区（命令结果、Webhook 提示）
- 底部：输入框

---

## 3. Webhook 接入（必做，否则待办不会自动更新）

Ops-Agent 在本机监听 HTTP，通过 **内嵌 Smee 隧道** 接收 GitHub 事件。**不需要**再单独运行 ngrok 或 `smee-client`。

### 3.1 流程概览

```
GitHub 仓库 Webhook
    ↓ POST
https://smee.io/你的频道ID        ← GitHub Payload URL（不要加 /webhook）
    ↓ 内嵌 Smee 客户端（ops-agent 进程内）
http://127.0.0.1:8765/webhooks/github   ← 本机 listen + path
    ↓
待办列表更新
```

### 3.2 步骤一：创建 smee 频道

1. 打开 [https://smee.io](https://smee.io)
2. 点击 **Start a new channel**
3. 复制频道 URL，形如：`https://smee.io/N6BMyoHea1WUggZM`
4. **注意：不要**在 URL 后面加 `/webhook`

### 3.3 步骤二：写入 Ops-Agent 配置

**方式 A：TUI 菜单（推荐）**

1. 启动 `./ops-agent`
2. 输入 `/webhook` 回车
3. **连接配置** → **Public URL** → 粘贴 smee 频道 URL → Enter 保存
4. 确认 **Smee 隧道** 为「已启用」
5. 按需修改 **监听地址**（默认 `127.0.0.1:8765`）和 **路径**（默认 `/webhooks/github`）

**方式 B：编辑 `.ops-agent.yaml`**

```yaml
webhook:
  enabled: true
  listen: "127.0.0.1:8765"
  path: "/webhooks/github"
  secret: ""
  public_url: "https://smee.io/YOUR-CHANNEL-ID"
  tunnel:
    smee:
      enabled: true
```

修改 yaml 后需**重启** `./ops-agent`。在 TUI 里改会自动保存并热重启 webhook。

### 3.4 步骤三：配置 GitHub Webhook（多仓库）

**推荐：Organization Webhook（一次配置，覆盖组织内所有仓库）**

路径：`GitHub → 你的 Organization → Settings → Webhooks → Add webhook`

| 字段 | 填什么 |
|------|--------|
| **Payload URL** | 与 `public_url` 相同，如 `https://smee.io/YOUR-CHANNEL-ID` |
| **Content type** | `application/json` |
| **Secret** | 与 `.ops-agent.yaml` 的 `webhook.secret` 一致；本地调试可都留空 |
| **Events** | 勾选 **Issues**、**Issue comments**、**Pull requests** |

> 待办列表按 Webhook payload 里的 `repository.full_name` 区分仓库（如 `tangjie133/test#34`），**与本地 cwd 无关**。状态栏显示「N 仓库」表示当前待办来自几个不同仓库。

若只监视个别仓库，也可在**每个仓库**的 Settings → Webhooks 里单独添加，Payload URL 填同一个 smee 频道即可。

**Events 说明：**

- **Issues**：新建（`opened`）、**关闭（`closed`）**、重新打开（`reopened`）
- **Issue comments**：评论触发入队（含历史 Issue/PR 讨论区）
- **Pull requests**：PR 关闭时从待办移除（与 Issues 关闭互补）

保存后 GitHub 会发 **ping**；TUI 输出区应出现 `Webhook: ping 成功`。

### 3.5 验证

**健康检查：**

```bash
curl http://127.0.0.1:8765/healthz
# 应返回 ok
```

**模拟 Issue 入队（不经过 GitHub）：**

```bash
WEBHOOK_URL=http://127.0.0.1:8765/webhooks/github make webhook-test
```

**真实测试：** 在仓库新建 Issue，或给已有 open Issue 评论，左侧待办应出现对应条目。

### 3.6 常见错误

| 现象 | 原因 | 处理 |
|------|------|------|
| GitHub delivery 404，`Server: uvicorn` | 别的服务占用了端口 | 用内嵌 smee，确认 `listen` 端口正确 |
| smee 404 `Cannot POST .../webhook` | Payload URL 多写了 `/webhook` | GitHub 只填 `https://smee.io/ID` |
| 收到事件但不入待办 | 未订阅 Issues | 勾选 Issues |
| 网页关闭 Issue 后待办仍在 | Webhook 未收到 `closed` 事件 | 用 **Organization Webhook**；确认订阅 Issues + Pull requests；在 GitHub Webhook 页查看 Recent Deliveries |
| `忽略事件 (issue_comment)` 且无入队 | Issue 已 closed | open Issue 评论才会入队 |
| `address already in use` | 端口被旧进程占用 | 关掉旧 `./ops-agent` 或改 `listen` |

---

## 4. 界面说明

### 4.1 待办列表（左侧）

每条待办**两行**显示：

```text
> ○ tangjie133/test#30
    技术支持
```

- 第一行：`owner/repo#编号`（多仓库时靠这个区分）
- 第二行：Issue 标题

### 4.2 输出区（右侧）

显示启动检查、Webhook 事件、`/issue` 详情等。输出过多时用 **`/clean`** 清空。

### 4.3 状态栏

示例：`qwen2.5-coder · manual · wh:on · tangjie133/Ops-Agent · 待办 3`

---

## 5. 快捷键与命令

### 5.1 快捷键（输入框为空时）

| 键 | 作用 |
|----|------|
| `j` / `k` | 待办下移 / 上移 |
| `i` | 查看选中 Issue 详情（**使用待办里的仓库**） |
| `d` | 忽略选中待办 |
| `Tab` / `→` | 命令补全 |
| `Ctrl+C` / `Esc` | 退出 |

### 5.2 斜杠命令

| 命令 | 说明 |
|------|------|
| `/help` | 帮助 |
| `/status` | 环境状态 |
| `/webhook` | Webhook / Smee / 入队规则（改完自动保存） |
| `/mode` | manual / semi / full |
| `/check` | 检测当前分支 PR（checks + 冲突） |
| `/issue owner/repo#n` | 查看 Issue；例：`/issue tangjie133/test#30` |
| `/clean` | 清空输出区 |

### 5.3 Issue 详情里的「未指派」

**指派: (未指派)** 表示 GitHub 上该 Issue **还没有 Assignee**，不是 Ops-Agent 的待办状态。  
**待办状态: in_todo** 才是 Ops-Agent 内部状态。

---

## 6. 待办何时增加 / 移除

| GitHub 事件 | 行为 |
|-------------|------|
| Issue **opened** | 符合规则 → 入待办 |
| **issue_comment**（新评论，Issue 仍 open） | 入队或重新激活 |
| Issue **closed** | 从待办移除（Webhook `issues`/`pull_request` closed） |
| Issue **reopened** | 重新入队 |

入队规则在 `/webhook` → **Issue 入队规则** 中配置（Label、仅未指派等）。**关闭/重开同步不走入队规则**，任意仓库的 closed 事件均会移除对应 `owner/repo#n` 待办。

---

## 7. 自动化模式（`/mode`）

| 模式 | 说明 |
|------|------|
| **manual** | 只展示待办（当前可用） |
| **semi** / **full** | AI 分析与回复（M3） |

---

## 8. 无 TUI 仅 Webhook

```bash
OPS_AGENT_WEBHOOK_ONLY=1 ./ops-agent
```

---

## 9. 交接清单

- [ ] `gh auth login` 完成
- [ ] `.ops-agent.yaml` 已配置 smee `public_url`
- [ ] `./ops-agent` 启动后 Smee「已连接」
- [ ] GitHub Payload URL = smee 频道（无 `/webhook`）
- [ ] 订阅 **Issues** + **Issue comments**
- [ ] 新建 Issue / 评论 / 关闭 各测一次
- [ ] 按 `i` 能打开正确仓库的 Issue

---

## 10. 更多参考

- 设计：`docs/DESIGN.md`
- 路线图：`docs/ROADMAP.md`
- 配置示例：`.ops-agent.yaml.example`

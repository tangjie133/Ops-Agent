# Ops-Agent 工程设计

> 技术接口与模块设计 · 对齐 README 架构

## 1. 进程入口

```go
cfg := config.Load()
if headless.ShouldRun() {
    os.Exit(headless.Run(cfg))
}
os.Exit(tui.Run(cfg))
```

### Headless 触发

- `OPS_AGENT_CI=1` 或 `CI=true`
- stdout 非 TTY

### Headless 分支

| 环境变量 | 行为 |
|----------|------|
| （默认） | PR 检测 → notify → exit |
| `OPS_AGENT_ISSUE_SCAN=1` | Issue 规则扫描 → notify |

## 2. 目录结构

```text
cmd/ops-agent/main.go
internal/
  config/       # YAML 加载
  github/       # gh exec 封装
  headless/     # CI 入口
  tui/          # Bubble Tea
  issuewatch/   # M2.5
  todo/         # M2.5
  worker/       # M2.5
  prcheck/      # M2
  notify/       # M2
  ai/           # M3
  agent/        # M3
```

## 3. 接口

### github.Client

```go
type Client interface {
    AuthStatus(ctx context.Context) (*AuthStatus, error)
    RepoFromCwd(ctx context.Context) (string, error)
    IssueList(ctx context.Context, opts IssueListOpts) ([]Issue, error)
    IssueView(ctx context.Context, repo string, num int) (*Issue, error)
    IssueComment(ctx context.Context, repo string, num int, body string) error
    PRView(ctx context.Context, repo string, num int) (*PullRequest, error)
    PRChecks(ctx context.Context, repo string, num int) (*ChecksResult, error)
}
```

### todo.Store（M2.5）

```go
type Status string // in_todo | analyzing | ready | posted | done | dismissed | failed

type Store interface {
    List() []Item
    Upsert(item Item) error
    Transition(repo string, num int, to Status) error
    SetDraft(repo string, num int, draft string) error
    Save() error
}
```

### notify.Notifier（M2）

```go
type Notifier interface {
    Send(ctx context.Context, alert Alert) error
}
```

---

## M2：PR 检测与通知

> 对齐 README §8。M2 **只做 PR**，与 Issue 待办（M2.5）完全分离。

### 范围

| 包含（M2 MVP） | 不包含（后续） |
|----------------|----------------|
| merge conflict 检测 | Required reviews / approvals |
| GitHub Actions checks 状态 | 分支落后 base |
| Slack / 飞书 / 钉钉并行 webhook | `gh pr comment` 留档 |
| TUI `/check` 展示报告 | TUI 内手动发 notify |
| CI headless：失败 notify + `exit 1` | Agent `github_pr_check` 工具（M3） |

### 模块职责

```text
internal/prcheck/
  check.go    # Check(ctx, gh, opts) → Result
  rules.go    # evaluate(pr) → []failure（纯函数，可单测）
  report.go   # FormatReport、ToAlert

internal/notify/
  alert.go    # Alert 结构与正文格式化
  multi.go    # 多通道并行 Send
  config.go   # FromAppConfig(cfg) 读 webhook

internal/headless/
  pr.go       # M2 默认路径：prcheck → notify → exit
  env.go      # GITHUB_EVENT_PATH / RUN_ID 解析
  issue.go    # 非 M2；OPS_AGENT_ISSUE_SCAN=1 时启用（M4 整理）
```

### 两条入口

| 入口 | 触发 | 行为 |
|------|------|------|
| **TUI** `/check` | 用户输入 | `prcheck.Check` → 仅打印报告，**不**发 notify |
| **Headless** | `OPS_AGENT_CI=1` / 非 TTY | 同上；失败且 `notify.on_failure` 时并行推送 → **`exit 1`** |

PR 解析：

- `GITHUB_REPOSITORY` + `GITHUB_EVENT_PATH.pull_request.number`（CI）
- 或 `OPS_AGENT_PR_NUMBER`（本地/调试）
- TUI / 本地：`RepoFromCwd` + 当前分支 PR

### 配置（M2 相关）

```yaml
notify:
  on_failure: true          # 仅 headless 失败时生效
  channels:
    slack:    { enabled, webhook_url: ${SLACK_WEBHOOK} }
    feishu:   { enabled, webhook_url: ${FEISHU_WEBHOOK} }
    dingtalk: { enabled, webhook_url: ${DINGTALK_WEBHOOK} }
```

`ci.pr_check_on_events` 由 workflow `on:` 控制，二进制内不解析。

### Alert 字段

| 字段 | 说明 |
|------|------|
| `title` | `[FAILED] PR #N checks` |
| `repo` | `org/repo` |
| `pr_number` / `pr_url` | PR 标识 |
| `failures` | 失败项列表 |
| `run_url` | Actions run 链接（CI） |

### 测试

| 包 | 策略 |
|----|------|
| prcheck | `evaluate` 纯函数表驱动 |
| notify | payload 格式 + httptest |
| headless | `env.go` 事件文件解析 |

---

## M2.5 步骤 1：Webhook 驱动 Issue 入待办

> **不再轮询单仓库**。与 GitHub App / 仓库 Webhook 协调：任意已安装仓库 `issues.opened` → 过滤 → 写 todo。

### 流程

```text
GitHub (App 或 repo webhook)
    POST /webhooks/github  (X-GitHub-Event: issues, action: opened)
        → 校验 X-Hub-Signature-256
        → issuewatch.Enqueue(repo from payload, issue)
        → todo.json 持久化
        → tea.Program.Send → TUI 待办面板刷新
```

### 模块

```text
internal/webhook/     server, handler, verify, payload
internal/issuewatch/  filter.go, enqueue.go
internal/todo/        store.go
```

### 配置

```yaml
issue_watch:
  enabled: true
  labels: ["ops"]
  require_unassigned: true
webhook:
  enabled: true
  listen: "127.0.0.1:8765"
  path: "/webhooks/github"
  secret: ${GITHUB_WEBHOOK_SECRET}
```

GitHub App：Issues 读权限 + 订阅 Issues 事件；Webhook URL 经 smee/ngrok 转发到本地 `listen+path`。

~~TUI 定时 gh list 轮询~~、~~headless issue scan~~ 已移除。

---

## 4. TUI 消息（Bubble Tea，M2.5+）

后台 goroutine 通过 `tea.Program.Send` 投递消息，**禁止在 Update 内阻塞**：

```go
type ScanCompleteMsg struct{ Items []todo.Item }
type WorkerUpdateMsg struct{ Item todo.Item }
type OutputMsg struct{ Text string }
```

## 5. Agent 工具协议（M3）

模型输出 JSON：

```json
{"tool":"github_issue_analyze","args":{"repo":"o/r","number":45}}
```

TUI 对话与 Issue Worker **共用** analyze 实现。

## 6. 配置

加载顺序：

1. `OPS_AGENT_CONFIG` 指定路径
2. 当前目录 `.ops-agent.yaml`
3. 用户目录 `~/.config/ops-agent/config.yaml`

环境变量 `${VAR}` 在加载后展开。

## 7. 依赖

```text
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
github.com/charmbracelet/lipgloss
gopkg.in/yaml.v3
golang.org/x/term
github.com/sashabaranov/go-openai  # M3
```

## 8. 测试

| 包 | 策略 |
|----|------|
| issuewatch | 纯函数过滤 |
| todo | 临时文件 |
| github | interface mock |
| notify | httptest |
| tui | Update 表驱动 |

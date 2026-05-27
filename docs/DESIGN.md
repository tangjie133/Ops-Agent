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

## 4. TUI 消息（Bubble Tea）

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

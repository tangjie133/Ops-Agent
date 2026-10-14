# Ops-Agent v1 代码范围 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让主工作区只剩下 v1 的 Issue/PR 闭环代码，并让这条闭环符合已确认的产品路线图。

**Architecture:** 进程只启动 TUI。TUI 打开期间，扫描或 webhook 把 Issue/PR 写入 `task.FileStore`，`orchestrator.RunIssueToPR` 在 Agent 自己的分支上改代码、跑该仓库已有测试、通过且非高风险时由 `publisher` 创建 draft PR。失败原因写入 `memory`，TUI 只显示任务阶段和该原因。v2 库验收和 v3 学习不在本计划中实现。

**Tech Stack:** Go、Bubble Tea、`gh` CLI、现有 `internal/task` JSON 存储。

**Spec:** `docs/superpowers/specs/2026-10-13-ops-agent-product-roadmap-design.md`

## Global Constraints

- 所有阶段都不自动合并 PR、关闭 Issue、force push。
- v1 只在 TUI 打开时入队并执行；不保留关掉 TUI 后继续跑的入口。
- 同一条 Issue 或 PR 的重复事件复用已有任务，不重复开分支、不重复开 PR。
- 已有 PR 改在 Agent 自己的新分支上，不改贡献者的分支。
- 仓库没有可运行的测试时停住，报告「缺少验证」，不把未测试当成通过。
- 高风险指本次 diff 删除了文件，或改动路径属于 CI 工作流、发布配置、权限或密钥。这类改动即使测试通过也不开 draft PR。
- 同一任务对同一目标连续 3 次测试失败后停止自动重试。
- 主工作区不得存在 `internal/libtest`、`internal/repovalidate`、`internal/knowledge`、`internal/skill`、`internal/budget`、`internal/feedback`、`internal/notify`、`internal/prcheck`、`internal/headless`。
- 允许留下的包若未被 v1 闭环调用，也删除。`.worktrees/` 移出主工作区。
- 不修改 `README.md`、`docs/DESIGN.md`、`docs/ROADMAP.md`、`docs/superpowers/specs/2026-10-13-ops-agent-task-orchestrator-design.md` 的正文。
- 旧计划 `docs/superpowers/plans/2026-10-13-issue-to-pr-orchestrator.md` 不再作为当前实现依据。

---

### Task 1: 代码范围守卫

**Files:**
- Create: `internal/scope/scope_test.go`
- Delete: `.worktrees/`
- Delete: `internal/libtest/` `internal/repovalidate/` `internal/knowledge/` `internal/skill/` `internal/budget/` `internal/feedback/` `internal/notify/` `internal/prcheck/` `internal/headless/`

**Interfaces:**
- Consumes: 路线图第 9 节的包名单
- Produces: `TestV1PackagesAbsent`，后续任务以它作为仓库范围的回归检查

- [ ] **Step 1: Write the failing test**

```go
package scope

import (
	"os"
	"path/filepath"
	"testing"
)

func TestV1PackagesAbsent(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	banned := []string{
		"internal/libtest",
		"internal/repovalidate",
		"internal/knowledge",
		"internal/skill",
		"internal/budget",
		"internal/feedback",
		"internal/notify",
		"internal/prcheck",
		"internal/headless",
		".worktrees",
	}
	for _, rel := range banned {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", rel)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scope -count=1`
Expected: FAIL，`internal/libtest still exists`

- [ ] **Step 3: Remove the banned trees**

```bash
rm -rf .worktrees \
  internal/libtest internal/repovalidate internal/knowledge internal/skill \
  internal/budget internal/feedback internal/notify internal/prcheck internal/headless
```

先不要修其它包的编译错误。本任务只删除这些目录。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scope -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scope/scope_test.go
git add -A internal .worktrees
git commit -m "$(cat <<'EOF'
Remove non-v1 packages so the tree matches the product roadmap.

EOF
)"
```

### Task 2: 入口只启动 TUI

**Files:**
- Modify: `cmd/ops-agent/main.go`
- Test: `internal/scope/scope_test.go`

**Interfaces:**
- Consumes: `config.Load`、`tui.Run`
- Produces: `main` 不再引用 `headless`

- [ ] **Step 1: Write the failing test**

把下面测试追加到 `internal/scope/scope_test.go`：

```go
func TestMainDoesNotImportHeadless(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "cmd", "ops-agent", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "internal/headless") {
		t.Fatal("main still imports headless")
	}
}
```

文件顶部的 import 增加 `"strings"`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scope -count=1 -run TestMainDoesNotImportHeadless`
Expected: FAIL，`main still imports headless`

- [ ] **Step 3: Write minimal implementation**

用下面内容替换 `cmd/ops-agent/main.go`：

```go
// Ops-Agent 主程序入口。v1 只启动 TUI；TUI 关闭后进程结束。
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := tui.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ops-agent: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scope -count=1 -run TestMainDoesNotImportHeadless`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/ops-agent/main.go internal/scope/scope_test.go
git commit -m "$(cat <<'EOF'
Start only the TUI from the process entry.

EOF
)"
```

### Task 3: 高风险与连续失败

**Files:**
- Modify: `internal/safety/evaluator.go`
- Modify: `internal/safety/evaluator_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `safety.Input` 字段 `Deleted int`、`Paths []string`、`VerifyOK bool`、`Retries int`；`Evaluate` 返回 `VerdictBlock`、`VerdictNeedsMoreFix`、`VerdictPublishAsDraft`

- [ ] **Step 1: Write the failing test**

用下面内容替换 `internal/safety/evaluator_test.go`：

```go
package safety

import "testing"

func TestEvaluateV1(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want Verdict
	}{
		{"any delete blocks", Input{Deleted: 1, VerifyOK: true}, VerdictBlock},
		{"workflow path blocks", Input{VerifyOK: true, Paths: []string{".github/workflows/ci.yml"}}, VerdictBlock},
		{"release path blocks", Input{VerifyOK: true, Paths: []string{".goreleaser.yaml"}}, VerdictBlock},
		{"secret path blocks", Input{VerifyOK: true, Paths: []string{"config/secrets.env"}}, VerdictBlock},
		{"third failure stops", Input{VerifyOK: false, Retries: 3}, VerdictBlock},
		{"earlier failure retries", Input{VerifyOK: false, Retries: 2}, VerdictNeedsMoreFix},
		{"clean pass publishes draft", Input{VerifyOK: true, Paths: []string{"internal/foo.go"}}, VerdictPublishAsDraft},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Evaluate(tc.in).Verdict; got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/safety -count=1`
Expected: FAIL，删除 1 个文件时期望 `block`，当前实现要删满 10 个才阻断

- [ ] **Step 3: Write minimal implementation**

用下面内容替换 `internal/safety/evaluator.go`：

```go
package safety

import "strings"

// Verdict 是安全评估结论。
type Verdict string

const (
	VerdictNeedsMoreFix   Verdict = "needs_more_fix"
	VerdictPublishAsDraft Verdict = "publish_as_draft"
	VerdictBlock          Verdict = "block"
)

// Input 是 v1 风险评估所需的事实。
type Input struct {
	Deleted  int
	Paths    []string
	VerifyOK bool
	Retries  int
}

// Report 是风险评估输出。
type Report struct {
	Verdict Verdict
	Reason  string
}

// Evaluate 按路线图第 3 节判断能否开 draft PR。
func Evaluate(in Input) Report {
	if in.Deleted > 0 {
		return Report{Verdict: VerdictBlock, Reason: "diff 删除了文件"}
	}
	for _, path := range in.Paths {
		if highRiskPath(path) {
			return Report{Verdict: VerdictBlock, Reason: "改动了高风险路径 " + path}
		}
	}
	if !in.VerifyOK {
		if in.Retries >= 3 {
			return Report{Verdict: VerdictBlock, Reason: "连续 3 次测试失败"}
		}
		return Report{Verdict: VerdictNeedsMoreFix, Reason: "测试失败"}
	}
	return Report{Verdict: VerdictPublishAsDraft, Reason: "测试通过"}
}

func highRiskPath(path string) bool {
	p := strings.ToLower(strings.TrimSpace(path))
	needles := []string{
		".github/workflows/",
		".goreleaser",
		"codeowners",
		"secret",
		".env",
		"credential",
		"/release",
	}
	for _, n := range needles {
		if strings.Contains(p, n) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/safety -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/safety/evaluator.go internal/safety/evaluator_test.go
git commit -m "$(cat <<'EOF'
Stop draft PRs on deletes, sensitive paths, and a third test failure.

EOF
)"
```

### Task 4: 没有测试就停住

**Files:**
- Modify: `internal/verifier/suite.go`
- Modify: `internal/verifier/suite_test.go`
- Modify: `internal/orchestrator/verify.go`

**Interfaces:**
- Consumes: 无
- Produces: `verifier.Run` 在 `Commands` 为空且未注入可执行命令时返回 `OK: false`、`Report: "缺少验证"`

- [ ] **Step 1: Write the failing test**

在 `internal/verifier/suite_test.go` 删除 `TestRunFailsOnMissingStandardFile`，追加：

```go
func TestRunStopsWhenNoTestCommand(t *testing.T) {
	res := Run(context.Background(), Opts{RepoPath: t.TempDir()})
	if res.OK || res.Report != "缺少验证" {
		t.Fatalf("ok=%v report=%q", res.OK, res.Report)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/verifier -count=1 -run TestRunStopsWhenNoTestCommand`
Expected: FAIL，当前空命令返回 `OK: true`

- [ ] **Step 3: Write minimal implementation**

从 `internal/verifier/suite.go` 删除 `repovalidate` 导入、`StandardsDir`、`Standard` 字段，以及 `opts.Standard` 分支。在 `Run` 开头、执行命令之前加入：

```go
if len(opts.Commands) == 0 {
	return Result{OK: false, Report: "缺少验证"}
}
```

把 `internal/orchestrator/verify.go` 的 `verifyRepo` 换成：

```go
func verifyRepo(ctx context.Context, cfg *config.Config, dir string) verifier.Result {
	_ = cfg
	opts := verifier.Opts{RepoPath: dir}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		opts.Commands = []string{"go test ./..."}
	}
	return verifier.Run(ctx, opts)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/verifier ./internal/orchestrator -count=1`
Expected: `./internal/verifier` PASS。`./internal/orchestrator` 若仍因 Task 1 删除的包而无法编译，留到 Task 5 再跑。

- [ ] **Step 5: Commit**

```bash
git add internal/verifier internal/orchestrator/verify.go
git commit -m "$(cat <<'EOF'
Treat a repository with no test command as unverified.

EOF
)"
```

### Task 5: 编排闭环去掉学习和预算包

**Files:**
- Modify: `internal/task/types.go`
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/orchestrator/orchestrator_test.go`
- Modify: `internal/orchestrator/enqueue.go`
- Modify: `internal/publisher/publisher.go`
- Modify: `internal/executor/internalagent/agent.go`（删除 `skill` 导入；提示词只用 Issue/PR 正文）

**Interfaces:**
- Consumes: `safety.Evaluate`、`verifier.Result`、`publisher.CreateDraftPR`、`task.FileStore.Ensure`
- Produces:
  - `task.SourceKindPR = "pr"`
  - `task.Task.StopReason string`
  - `orchestrator.Deps` 不再含 `Policy`、`KnowledgePath`
  - `orchestrator.EnqueuePR(cfg *config.Config, store *task.FileStore, repo string, number int, title, url string) (bool, task.Task, error)`
  - `publisher.CommentDraftPR(ctx, repo, prURL, body, taskID, runID string) error`

- [ ] **Step 1: Write the failing test**

在 `internal/orchestrator/orchestrator_test.go` 去掉 `budget` 导入。追加测试，使用现有 fake executor：验证连续三次 `Verify` 返回 `OK: false` 后，任务 `StopReason` 为 `连续 3 次测试失败`，且 `Publish.CreateDraftPR` 未被调用。再追加一个测试：`Verify` 返回 `OK: true` 且 diff 为空时创建 draft PR，并调用 `CommentDraftPR`。

测试里的 `Deps` 写成：

```go
deps := Deps{
	Tasks: store,
	Exec:  exec,
	Publish: publisher.New(fakeGH),
	MemoryPath: filepath.Join(dir, "memory.jsonl"),
	Verify: func(ctx context.Context, work string) verifier.Result {
		return verifier.Result{OK: false, Report: "fail"}
	},
}
```

`Enqueue` 测试：

```go
func TestEnqueuePRReusesTask(t *testing.T) {
	store, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Task.IssueToPR.Enabled = true
	_, first, err := EnqueuePR(cfg, store, "o/r", 7, "title", "url")
	if err != nil {
		t.Fatal(err)
	}
	created, second, err := EnqueuePR(cfg, store, "o/r", 7, "title", "url")
	if err != nil {
		t.Fatal(err)
	}
	if created || second.ID != first.ID || second.SourceKind != task.SourceKindPR {
		t.Fatalf("created=%v first=%s second=%+v", created, first.ID, second)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator -count=1 -run 'TestEnqueuePRReusesTask|TestStopsAfterThirdVerifyFailure'`
Expected: FAIL，`EnqueuePR` 或 `SourceKindPR` 未定义

- [ ] **Step 3: Write minimal implementation**

在 `internal/task/types.go` 增加：

```go
const SourceKindPR SourceKind = "pr"
```

给 `Task` 增加字段：

```go
StopReason string `json:"stop_reason,omitempty"`
```

`EnqueuePR`：

```go
func EnqueuePR(cfg *config.Config, store *task.FileStore, repo string, number int, title, url string) (bool, task.Task, error) {
	if cfg == nil || !cfg.Task.IssueToPR.Enabled {
		return false, task.Task{}, nil
	}
	if store == nil {
		return false, task.Task{}, fmt.Errorf("task store required")
	}
	return store.Ensure(task.Task{
		Repo: repo, Type: task.TypeIssueToPR, SourceKind: task.SourceKindPR,
		SourceNumber: number, Title: title, URL: url,
	})
}
```

`orchestrator.go` 删除 `budget`、`feedback`、`knowledge`、`skill` 导入以及 `knowledgePath`、`allow`、`feedbackCfg`。`Deps` 删除 `Policy` 和 `KnowledgePath`。循环里用局部 `retries`：验证失败时调用 `safety.Evaluate(safety.Input{Deleted: deleted, Paths: paths, VerifyOK: false, Retries: retries + 1})`；`VerdictNeedsMoreFix` 才继续，并把 `retries++`。第三次失败走 `stop`，把 `report.Reason` 写入 `task.StopReason` 和 memory。成功路径不再进入 `StatusLearning`，直接 `StatusDone`。分支名：Issue 用 `ops-agent/issue-%d`，PR 用 `ops-agent/pr-%d`。`diffStats` 同时返回变更路径，用 `git -C dir diff --name-only HEAD` 与未跟踪文件；删除行仍用 `git status --porcelain` 里以 `D` 开头的条目。

`CommentDraftPR` 从 PR URL 最后一个 `/` 之后解析数字，调用已有 `GitHubAPI.IssueComment`。创建 draft PR 成功后立刻调用它，正文为 `req.Body`。

`internal/executor/internalagent/agent.go` 删除 `skill.Directive()` 调用，提示词只保留任务正文。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator ./internal/publisher ./internal/executor ./internal/task -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task internal/orchestrator internal/publisher internal/executor
git commit -m "$(cat <<'EOF'
Run the issue and PR loop without learning or budget packages.

EOF
)"
```

### Task 6: TUI 只显示 v1 任务

**Files:**
- Create: `internal/tui/task_line.go`
- Create: `internal/tui/task_line_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/run.go`
- Modify: `internal/tui/webhook_runtime.go`
- Delete: `internal/tui/libtest_ui.go` `internal/tui/test_panel.go` `internal/tui/accept_menu.go` `internal/tui/accept_menu_test.go`
- Modify: 仍引用 `libtest`、`libTestStore`、`testSel`、`acceptMenu` 的 `internal/tui/*_test.go`，删除这些断言和构造参数

**Interfaces:**
- Consumes: `task.Task` 的 `SourceKind`、`SourceNumber`、`Status`、`StopReason`、`PRURL`
- Produces: `FormatTaskLine(tk task.Task) string`；`NewModel(cfg *config.Config, tasks *task.FileStore, wh *WebhookRuntime) Model`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestFormatTaskLineShowsStopReason(t *testing.T) {
	line := FormatTaskLine(task.Task{
		SourceKind: task.SourceKindIssue, SourceNumber: 45,
		Status: task.StatusNeedsManualIntervention, StopReason: "连续 3 次测试失败",
	})
	if !strings.Contains(line, "#45") || !strings.Contains(line, "连续 3 次测试失败") {
		t.Fatalf("line=%q", line)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -count=1 -run TestFormatTaskLineShowsStopReason`
Expected: FAIL，`FormatTaskLine` 未定义

- [ ] **Step 3: Write minimal implementation**

```go
package tui

import (
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// FormatTaskLine 渲染一条 v1 任务，供左侧列表显示阶段和停住原因。
func FormatTaskLine(tk task.Task) string {
	line := fmt.Sprintf("%s #%d %s", tk.SourceKind, tk.SourceNumber, tk.Status)
	if tk.StopReason != "" {
		line += " " + tk.StopReason
	}
	if tk.PRURL != "" {
		line += " " + tk.PRURL
	}
	return line
}
```

然后：

- `NewModel` 去掉 `libTestStore` 参数和字段。
- `run.go`、`webhook_runtime.go` 去掉 `libtest` 导入和对应参数。
- 删除只服务库验收的 `libtest_ui.go`、`test_panel.go`、`accept_menu.go`、`accept_menu_test.go`。
- 左侧列表现有待办渲染改为对 `tasks.List()` 的每一项调用 `FormatTaskLine`。
- 从 `model.go` 删除字段 `libTestStore`、`libTestBusy`、`testSel`、`testAnchorRepo`、`testAnchorRef`、`acceptMenuOpen`、`acceptMenuSel`、`invLogSink`、`invStatus`、`investigatorLogFn`。删除后编译器报出的同名引用一并删除，不保留库验收或调查日志界面。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -count=1 -run TestFormatTaskLineShowsStopReason`
Expected: PASS

随后运行 `go test ./internal/tui -count=1`。失败则只删除对已删符号的引用，不恢复库验收界面。

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "$(cat <<'EOF'
Show v1 task stage and stop reason in the TUI.

EOF
)"
```

### Task 7: 扫描和 webhook 入队，并恢复整仓编译

**Files:**
- Modify: `internal/webhook/handler.go`
- Modify: `internal/webhook/server.go`
- Modify: `internal/webhook/runtime.go`
- Delete: `internal/webhook/libtest.go`
- Modify: `internal/webhook/handler_test.go`
- Modify: `internal/tui/task_worker.go`
- Modify: `internal/tui/poll_ui.go` 与 `internal/tui/todo_panel_test.go`（去掉 `libtest`）

**Interfaces:**
- Consumes: `orchestrator.EnqueueIssue`、`orchestrator.EnqueuePR`
- Produces: webhook 的 Issue 与 Pull Request 事件都调用对应 Enqueue；TUI 轮询扫描同样调用它们。不存在 `libtest` 符号。

- [ ] **Step 1: Write the failing test**

在 `internal/webhook/handler_test.go` 删除所有 `libtest` 导入和用例。`NewHandler` 现有签名是 `NewHandler(cfg, tasks, libTest, onEvent, logger)`。测试先按删除 `libTest` 之后的签名编写，因此会编译失败：

```go
func TestIssueEventEnqueuesTask(t *testing.T) {
	store, err := task.Load(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Task.IssueToPR.Enabled = true
	h := NewHandler(cfg, store, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"action":"opened","issue":{"number":45,"title":"bug","html_url":"http://example/45"},"repository":{"full_name":"o/r"}}`))
	req.Header.Set("X-GitHub-Event", "issues")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	items := store.List()
	if len(items) != 1 || items[0].SourceNumber != 45 || items[0].SourceKind != task.SourceKindIssue {
		t.Fatalf("tasks=%+v", items)
	}
}

func TestPullRequestEventEnqueuesTask(t *testing.T) {
	store, err := task.Load(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Task.IssueToPR.Enabled = true
	h := NewHandler(cfg, store, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"action":"opened","pull_request":{"number":9,"title":"fix","html_url":"http://example/9"},"repository":{"full_name":"o/r"}}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	items := store.List()
	if len(items) != 1 || items[0].SourceNumber != 9 || items[0].SourceKind != task.SourceKindPR {
		t.Fatalf("status=%d tasks=%+v", rec.Code, items)
	}
}
```

测试文件需要 `net/http`、`net/http/httptest`、`strings`。实现完成后 `NewHandler` 只有四个参数：`cfg`、`tasks`、`onEvent`、`logger`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webhook -count=1 -run TestIssueEventEnqueuesTask`
Expected: FAIL，编译错误指向 `libtest`，或事件没有入队

- [ ] **Step 3: Write minimal implementation**

- 删除 `internal/webhook/libtest.go`。
- `NewHandler` 改为 `NewHandler(cfg *config.Config, tasks *task.FileStore, onEvent OnEvent, logger *log.Logger) *Handler`，删除 `libTest` 字段。`server.go`、`runtime.go` 的调用同步改掉。
- Issue 的 `opened` 继续调用 `orchestrator.EnqueueIssue`。`handlePullRequest` 在 `action` 为 `opened` 时调用 `orchestrator.EnqueuePR`，不再直接返回 skipped。`push`、`release`、`repository` 这些只服务库验收的分支删除，未知事件保持 200 且不入队。
- `internal/tui/task_worker.go` 去掉 `budget` 和 `headless` 导入。构造 `orchestrator.Deps` 时不填 `Policy` 和 `KnowledgePath`。
- `poll_ui.go` 的扫描命中 Issue 时调用 `EnqueueIssue`，命中 PR 时调用 `EnqueuePR`。不要在扫描函数里等待人工确认。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/webhook ./internal/tui ./internal/orchestrator -count=1`
Expected: PASS

然后：

Run: `go test ./...`
Expected: PASS

Run: `rg -n 'internal/(libtest|repovalidate|knowledge|skill|budget|feedback|notify|prcheck|headless)' --glob '*.go' -g '!.worktrees/**'`
Expected: 无匹配

- [ ] **Step 5: Commit**

```bash
git add internal/webhook internal/tui internal/orchestrator
git commit -m "$(cat <<'EOF'
Enqueue issues and pull requests while the TUI is open.

EOF
)"
```

## Self-review

- 路线图第 3 节的入队、自己的分支、测试、draft PR、重复事件、TUI 停住原因、3 次失败、高风险、缺少验证，分别由 Task 5、6、3、4、7 覆盖。
- 第 6 节禁止的合并、关闭、force push、Web 后台、常驻进程、库验收、学习，由删除包和 publisher 只保留 draft PR 与评论覆盖。
- 第 9 节的删除名单和 `.worktrees` 由 Task 1 的测试锁住。
- v2、v3 没有实现任务。
- 不改四份旧文档正文。

## Execution note

实现时不要把 v2 库验收或 v3 学习的代码留在主工作区里备用。

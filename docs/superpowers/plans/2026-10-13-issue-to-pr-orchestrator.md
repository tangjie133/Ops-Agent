# Issue → Draft PR 编排闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 Ops-Agent 上落地第一轮 `issue_to_pr` 闭环：幂等入队、独立 worktree、预算限制、内置执行器改代码、本地验证、风险判断、创建 draft PR，并记录 Memory。

**Architecture:** 新增 Task 编排核心，旧 `todo`/`worker`/`refactor` 本轮不删除。Webhook 在命中 Issue 后双写 Task store；`orchestrator` 用接口驱动 executor / verifier / safety / publisher。GitHub 写回只走 publisher，默认 draft PR，不 merge、不 close issue。

**Tech Stack:** Go 1.22、现有 `gh` 封装、`gopkg.in/yaml.v3`、标准库 `testing`、本地 git worktree。不新增第三方依赖。

## Global Constraints

- 语言：Go 1.22；模块路径 `github.com/ZzedJay/Ops-Agent`。
- 新代码必须有完整中文注释（包注释、导出类型、导出函数）。
- 所有 GitHub 读写继续走 `internal/github`；业务策略不写进 github 包。
- 插件不得直接写 GitHub。
- 默认不自动 merge、不自动 close issue、不做破坏性回滚。
- 第一轮不实现完整插件系统、完整 Arduino CLI matrix、完整 CI/review/bot 分类、`max_estimated_cost` 阻断、自动更新 Skill、高级 TUI。
- 测试命令：`go test ./internal/<pkg> -count=1`；全量：`go test ./... -count=1`。
- 提交信息用英文 `feat:` / `test:` / `docs:` 前缀，每次任务结束提交一次。
- 旧 `todo` 队列继续工作；新路径由 `task.issue_to_pr.enabled` 控制，默认 `false`。

## File Map

```text
internal/task/types.go              # Task/Run/Status/Type
internal/task/key.go                # TaskKey / RunKey
internal/task/store.go              # JSON 持久化
internal/task/transition.go         # 合法状态流转
internal/config/task.go             # workspace/budget/feedback/task YAML
internal/budget/policy.go           # 预算默认值
internal/budget/meter.go            # 计量与 Allow
internal/feedback/classifier.go     # 第一轮最小分类桩
internal/workspace/manager.go       # worktree 分配/清理
internal/memory/store.go            # Task 事件日志
internal/verifier/suite.go          # repo_validate + 命令
internal/safety/evaluator.go        # diff/测试风险结论
internal/executor/types.go          # Executor 接口
internal/executor/internalagent/adapter.go  # 包装 refactor.Engine
internal/publisher/publisher.go     # draft PR + footer
internal/skill/issue_to_pr.go       # 流程护栏文本
internal/orchestrator/orchestrator.go
internal/orchestrator/enqueue.go    # Webhook/TUI 入队
```

修改：

```text
internal/config/config.go           # 挂新配置并 Normalize
.ops-agent.yaml.example             # 追加 task/workspace/budget/feedback
internal/webhook/handler.go         # Issue 命中后 EnsureTask
```

---

### Task 1: TaskKey / RunKey

**Files:**
- Create: `internal/task/key.go`
- Create: `internal/task/key_test.go`
- Create: `internal/task/doc.go`

**Interfaces:**
- Consumes: 无
- Produces: `MakeTaskKey(repo string, typ Type, kind SourceKind, number int) string`；`MakeRunKey(taskKey, triggerHash string) string`；常量 `TypeIssueToPR`、`SourceKindIssue`

- [ ] **Step 1: Write the failing test**

```go
package task

import "testing"

func TestMakeTaskKeyStable(t *testing.T) {
	a := MakeTaskKey("Owner/Repo", TypeIssueToPR, SourceKindIssue, 45)
	b := MakeTaskKey("owner/repo", TypeIssueToPR, SourceKindIssue, 45)
	if a != b {
		t.Fatalf("expected case-insensitive repo, %q vs %q", a, b)
	}
	if a != "owner/repo|issue_to_pr|issue|45" {
		t.Fatalf("got %q", a)
	}
}

func TestMakeRunKeyIncludesTrigger(t *testing.T) {
	tk := MakeTaskKey("o/r", TypeIssueToPR, SourceKindIssue, 1)
	r1 := MakeRunKey(tk, "comment:99")
	r2 := MakeRunKey(tk, "comment:99")
	r3 := MakeRunKey(tk, "comment:100")
	if r1 != r2 {
		t.Fatal("same trigger must match")
	}
	if r1 == r3 {
		t.Fatal("different trigger must differ")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -count=1 -run 'TestMakeTaskKeyStable|TestMakeRunKeyIncludesTrigger'`

Expected: FAIL，包或符号未定义。

- [ ] **Step 3: Write minimal implementation**

`internal/task/doc.go`：

```go
// Package task 定义统一任务模型、幂等键、状态机与持久化。
package task
```

`internal/task/types.go` 先放常量：

```go
package task

// Type 是任务种类。第一轮只实现 issue_to_pr。
type Type string

const TypeIssueToPR Type = "issue_to_pr"

// SourceKind 标识任务来源对象。
type SourceKind string

const SourceKindIssue SourceKind = "issue"
```

`internal/task/key.go`：

```go
package task

import (
	"fmt"
	"strings"
)

// MakeTaskKey 生成稳定任务键：同一仓库+类型+来源编号复用同一 Task。
func MakeTaskKey(repo string, typ Type, kind SourceKind, number int) string {
	repo = strings.ToLower(strings.TrimSpace(repo))
	return fmt.Sprintf("%s|%s|%s|%d", repo, typ, kind, number)
}

// MakeRunKey 生成单次执行键：同一 Task 的同一触发内容不重复跑。
func MakeRunKey(taskKey, triggerHash string) string {
	return taskKey + "|" + strings.TrimSpace(triggerHash)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/task -count=1 -run 'TestMakeTaskKeyStable|TestMakeRunKeyIncludesTrigger'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task/doc.go internal/task/types.go internal/task/key.go internal/task/key_test.go
git commit -m "feat: add task idempotency keys"
```

---

### Task 2: Task store 与状态流转

**Files:**
- Modify: `internal/task/types.go`
- Create: `internal/task/transition.go`
- Create: `internal/task/store.go`
- Create: `internal/task/store_test.go`

**Interfaces:**
- Consumes: `MakeTaskKey`、`MakeRunKey`
- Produces: `FileStore`：`Ensure(Task) (created bool, t Task, err error)`、`BeginRun(taskKey, triggerHash string) (run Run, skipped bool, err error)`、`Transition(id string, to Status) error`、`GetByKey(key string) (Task, bool)`、`SetActiveRun(taskID, runID string) error`

状态常量必须与 spec 一致：

```go
const (
	StatusCreated                 Status = "created"
	StatusCollectingContext       Status = "collecting_context"
	StatusPlanning                Status = "planning"
	StatusAwaitingApproval        Status = "awaiting_approval"
	StatusExecuting               Status = "executing"
	StatusVerifying               Status = "verifying"
	StatusEvaluatingSafety        Status = "evaluating_safety"
	StatusAwaitingHumanReview     Status = "awaiting_human_review"
	StatusPublishing              Status = "publishing"
	StatusLearning                Status = "learning"
	StatusDone                    Status = "done"
	StatusFailed                  Status = "failed"
	StatusCancelled               Status = "cancelled"
	StatusRolledBack              Status = "rolled_back"
	StatusNeedsManualIntervention Status = "needs_manual_intervention"
)
```

合法后继（实现 `CanTransition(from, to Status) bool`）：

- `created` → `collecting_context` / `cancelled`
- 主链按 spec 顺序前进
- 任意非终态 → `failed` / `cancelled` / `rolled_back` / `needs_manual_intervention`
- 终态：`done`、`cancelled`、`rolled_back` 不可再转，除非从 `failed`/`needs_manual_intervention` 回到 `collecting_context`（人工重跑）

- [ ] **Step 1: Write the failing test**

```go
package task

import (
	"path/filepath"
	"testing"
)

func TestEnsureReusesTaskKey(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	in := Task{Repo: "o/r", Type: TypeIssueToPR, SourceKind: SourceKindIssue, SourceNumber: 45, Title: "bug"}
	created, t1, err := s.Ensure(in)
	if err != nil || !created {
		t.Fatalf("first ensure created=%v err=%v", created, err)
	}
	created, t2, err := s.Ensure(in)
	if err != nil || created {
		t.Fatalf("second ensure created=%v err=%v", created, err)
	}
	if t1.ID != t2.ID {
		t.Fatalf("ids %s vs %s", t1.ID, t2.ID)
	}
}

func TestBeginRunSkipsDuplicateTrigger(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "tasks.json"))
	_, tk, _ := s.Ensure(Task{Repo: "o/r", Type: TypeIssueToPR, SourceKind: SourceKindIssue, SourceNumber: 1})
	r1, skipped, err := s.BeginRun(tk.Key, "wh:1")
	if err != nil || skipped {
		t.Fatalf("first run skipped=%v err=%v", skipped, err)
	}
	_, skipped, err = s.BeginRun(tk.Key, "wh:1")
	if err != nil || !skipped {
		t.Fatalf("dup run skipped=%v err=%v", skipped, err)
	}
	if r1.ID == "" {
		t.Fatal("empty run id")
	}
}

func TestBeginRunRejectsParallel(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "tasks.json"))
	_, tk, _ := s.Ensure(Task{Repo: "o/r", Type: TypeIssueToPR, SourceKind: SourceKindIssue, SourceNumber: 2})
	if _, skipped, err := s.BeginRun(tk.Key, "a"); err != nil || skipped {
		t.Fatal(err)
	}
	_, skipped, err := s.BeginRun(tk.Key, "b")
	if err == nil || skipped {
		t.Fatalf("expected parallel error, skipped=%v err=%v", skipped, err)
	}
}

func TestTransitionRejectsIllegal(t *testing.T) {
	if CanTransition(StatusDone, StatusExecuting) {
		t.Fatal("done must be terminal")
	}
	if !CanTransition(StatusCreated, StatusCollectingContext) {
		t.Fatal("created -> collecting_context")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -count=1`

Expected: FAIL，`Load`/`Ensure` 未定义。

- [ ] **Step 3: Write minimal implementation**

在 `types.go` 补全：

```go
package task

import "time"

type Status string
// 常量见本任务开头列表

type Task struct {
	ID            string     `json:"id"`
	Key           string     `json:"key"`
	Type          Type       `json:"type"`
	Status        Status     `json:"status"`
	Repo          string     `json:"repo"`
	SourceKind    SourceKind `json:"source_kind"`
	SourceNumber  int        `json:"source_number"`
	Title         string     `json:"title,omitempty"`
	URL           string     `json:"url,omitempty"`
	Branch        string     `json:"branch,omitempty"`
	PRURL         string     `json:"pr_url,omitempty"`
	WorkspacePath string     `json:"workspace_path,omitempty"`
	ActiveRunID   string     `json:"active_run_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Run struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	Key         string    `json:"key"`
	TriggerHash string    `json:"trigger_hash"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
```

`store.go` 模仿 `internal/todo/store.go`：JSON 数组、`sync.RWMutex`、`Load`/`Save`。`Ensure` 用 `MakeTaskKey` 查找；不存在则分配 `id`（`time.Now().UTC().Format("20060102T150405.000000000")` 加随机 4 位不足时可用 `fmt.Sprintf("%s#%d-%d", repo, n, time.Now().UnixNano())`）。`BeginRun`：若 `ActiveRunID` 非空则返回 error `并行 Run 未结束`；若已有相同 `Run.Key` 则 `skipped=true`；否则创建 Run 并写入 `ActiveRunID`。`ClearActiveRun(taskID string) error` 在 Run 结束时调用。

`transition.go`：实现 `CanTransition` 与 `FileStore.Transition`。非法流转返回 `fmt.Errorf("illegal transition %s -> %s", from, to)`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/task -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task
git commit -m "feat: persist tasks with status transitions and run dedupe"
```

---

### Task 3: 配置 Normalize

**Files:**
- Create: `internal/config/task.go`
- Create: `internal/config/task_test.go`
- Modify: `internal/config/config.go`（`Config` 增加字段，`Default`/`Load` 调用 Normalize）
- Modify: `.ops-agent.yaml.example`

**Interfaces:**
- Consumes: 无
- Produces: `config.TaskConfig`、`WorkspaceConfig`、`BudgetConfig`、`FeedbackConfig`；`(*TaskConfig).Normalize()`；`(*BudgetConfig).Normalize()`

- [ ] **Step 1: Write the failing test**

```go
package config

import "testing"

func TestTaskConfigDefaults(t *testing.T) {
	var c TaskConfig
	c.Normalize()
	if !c.Idempotency {
		t.Fatal("idempotency default true")
	}
	if c.IssueToPR.Enabled {
		t.Fatal("issue_to_pr must default off")
	}
	if c.Workspace.Mode != "worktree" {
		t.Fatalf("mode=%s", c.Workspace.Mode)
	}
	if c.Workspace.CleanupAfter != "72h" {
		t.Fatalf("cleanup=%s", c.Workspace.CleanupAfter)
	}
	if !c.Workspace.KeepFailed {
		t.Fatal("keep_failed")
	}
	if c.Budget.MaxSteps != 12 || c.Budget.MaxRetries != 3 || c.Budget.MaxDuration != "30m" || c.Budget.MaxCommands != 20 || c.Budget.MaxLLMCalls != 12 {
		t.Fatalf("budget %+v", c.Budget)
	}
	if !c.Feedback.IgnoreKnownBotNoise || !c.Feedback.EscalateOnUnknownReview {
		t.Fatalf("feedback %+v", c.Feedback)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -count=1 -run TestTaskConfigDefaults`

Expected: FAIL，`TaskConfig` 未定义。

- [ ] **Step 3: Write minimal implementation**

`internal/config/task.go`：

```go
package config

// TaskConfig 统一任务编排相关配置。
type TaskConfig struct {
	StorePath   string          `yaml:"store_path"`
	Idempotency bool            `yaml:"idempotency"`
	IssueToPR   IssueToPRConfig `yaml:"issue_to_pr"`
	Workspace   WorkspaceConfig `yaml:"workspace"`
	Budget      BudgetConfig    `yaml:"budget"`
	Feedback    FeedbackConfig  `yaml:"feedback"`
}

type IssueToPRConfig struct {
	Enabled bool `yaml:"enabled"`
}

type WorkspaceConfig struct {
	Mode         string `yaml:"mode"`
	KeepFailed   bool   `yaml:"keep_failed"`
	CleanupAfter string `yaml:"cleanup_after"`
	Root         string `yaml:"root"`
}

type BudgetConfig struct {
	MaxSteps     int    `yaml:"max_steps"`
	MaxRetries   int    `yaml:"max_retries"`
	MaxDuration  string `yaml:"max_duration"`
	MaxCommands  int    `yaml:"max_commands"`
	MaxLLMCalls  int    `yaml:"max_llm_calls"`
}

type FeedbackConfig struct {
	IgnoreKnownBotNoise      bool `yaml:"ignore_known_bot_noise"`
	EscalateOnUnknownReview  bool `yaml:"escalate_on_unknown_review"`
}

func (c *TaskConfig) Normalize() {
	c.Idempotency = true
	if c.Workspace.Mode == "" {
		c.Workspace.Mode = "worktree"
	}
	if c.Workspace.CleanupAfter == "" {
		c.Workspace.CleanupAfter = "72h"
	}
	if !c.Workspace.KeepFailed && c.Workspace.CleanupAfter != "" {
		c.Workspace.KeepFailed = true
	}
	// KeepFailed：零值时默认 true。用独立标记避免无法关闭。
	// 实现：若 YAML 显式 false，允许关闭。Default() 里设 true。
	if c.Budget.MaxSteps <= 0 {
		c.Budget.MaxSteps = 12
	}
	if c.Budget.MaxRetries <= 0 {
		c.Budget.MaxRetries = 3
	}
	if c.Budget.MaxDuration == "" {
		c.Budget.MaxDuration = "30m"
	}
	if c.Budget.MaxCommands <= 0 {
		c.Budget.MaxCommands = 20
	}
	if c.Budget.MaxLLMCalls <= 0 {
		c.Budget.MaxLLMCalls = 12
	}
}
```

注意：`KeepFailed` 默认 true 与“显式 false”冲突。实现约定：`Default()` 设 `KeepFailed: true`；`Normalize()` **不要**把 false 改回 true。测试里对零值结构体先在 `Default` 路径覆盖。把测试改为：

```go
func TestTaskConfigDefaults(t *testing.T) {
	cfg := Default()
	cfg.Task.Normalize()
	...
}
```

并在 `Default()` 里设置 `KeepFailed: true`、`Feedback` 两个 bool 为 true。`Normalize` 只填空字符串和 `<=0` 数字。

在 `Config` 增加 `Task TaskConfig `yaml:"task"``。`Load()` 在其它 Normalize 之后调用 `cfg.Task.Normalize()`。

`.ops-agent.yaml.example` 追加 spec 中的 yaml 片段，并加：

```yaml
task:
  idempotency: true
  issue_to_pr:
    enabled: false
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config -count=1 -run TestTaskConfigDefaults`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config .ops-agent.yaml.example
git commit -m "feat: add task workspace budget and feedback config"
```

---

### Task 4: Budget meter

**Files:**
- Create: `internal/budget/doc.go`
- Create: `internal/budget/policy.go`
- Create: `internal/budget/meter.go`
- Create: `internal/budget/meter_test.go`

**Interfaces:**
- Consumes: `config.BudgetConfig`
- Produces: `FromConfig(cfg config.BudgetConfig) (Policy, error)`；`(*Meter).Allow(p Policy, now time.Time) bool`；`(*Meter).NoteStep()` / `NoteCommand()` / `NoteLLM()` / `NoteRetry()`

- [ ] **Step 1: Write the failing test**

```go
package budget

import (
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestMeterBlocksWhenStepsExceeded(t *testing.T) {
	p, err := FromConfig(config.BudgetConfig{MaxSteps: 2, MaxRetries: 3, MaxDuration: "30m", MaxCommands: 20, MaxLLMCalls: 12})
	if err != nil {
		t.Fatal(err)
	}
	m := NewMeter(time.Unix(0, 0))
	m.NoteStep()
	if !m.Allow(p, time.Unix(1, 0)) {
		t.Fatal("first step allowed")
	}
	m.NoteStep()
	if m.Allow(p, time.Unix(2, 0)) {
		t.Fatal("third step must block")
	}
}

func TestMeterBlocksWhenDurationExceeded(t *testing.T) {
	p, err := FromConfig(config.BudgetConfig{MaxSteps: 12, MaxRetries: 3, MaxDuration: "1s", MaxCommands: 20, MaxLLMCalls: 12})
	if err != nil {
		t.Fatal(err)
	}
	m := NewMeter(time.Unix(0, 0))
	if m.Allow(p, time.Unix(2, 0)) {
		t.Fatal("duration exceeded")
	}
}

func TestFromConfigRejectsBadDuration(t *testing.T) {
	_, err := FromConfig(config.BudgetConfig{MaxSteps: 1, MaxDuration: "abc"})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/budget -count=1`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`Allow` 在 **下一次动作前** 检查：若 `Steps >= MaxSteps` 或 `Commands >= MaxCommands` 或 `LLMCalls >= MaxLLMCalls` 或 `Retries >= MaxRetries` 或 `now.Sub(Started) > MaxDuration` 则 false。`Note*` 只增加计数。成本字段本轮只存在于 Meter 作为 `EstimatedCost float64`，`Allow` 忽略它。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/budget -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/budget
git commit -m "feat: add task budget meter"
```

---

### Task 5: Feedback 最小分类桩

**Files:**
- Create: `internal/feedback/doc.go`
- Create: `internal/feedback/classifier.go`
- Create: `internal/feedback/classifier_test.go`

**Interfaces:**
- Consumes: `config.FeedbackConfig`
- Produces: `Classify(cfg config.FeedbackConfig, sig Signal) Kind`

```go
type Kind string
const (
	KindFixCode     Kind = "fix_code"
	KindFixTest     Kind = "fix_test"
	KindFixConfig   Kind = "fix_config"
	KindIgnoreNoise Kind = "ignore_noise"
	KindWait        Kind = "wait"
	KindEscalate    Kind = "escalate"
)

type Signal struct {
	Source       string // verifier | github_comment
	VerifyFailed bool
	Text         string
}
```

规则（写死，本轮不解析 CI）：

1. `Source=="verifier"` 且 `VerifyFailed`：若 Text 含 `demo`/`test`/`go test` 则 `fix_test`，否则 `fix_code`。
2. `Source=="github_comment"`：若 `EscalateOnUnknownReview` 则为 `escalate`。
3. 其它：`escalate`。

- [ ] **Step 1: Write the failing test**

```go
package feedback

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestClassifyVerifierFailure(t *testing.T) {
	cfg := config.FeedbackConfig{EscalateOnUnknownReview: true}
	if Classify(cfg, Signal{Source: "verifier", VerifyFailed: true, Text: "go test failed"}) != KindFixTest {
		t.Fatal("expected fix_test")
	}
	if Classify(cfg, Signal{Source: "verifier", VerifyFailed: true, Text: "missing README"}) != KindFixCode {
		t.Fatal("expected fix_code")
	}
}

func TestClassifyUnknownCommentEscalates(t *testing.T) {
	cfg := config.FeedbackConfig{EscalateOnUnknownReview: true}
	if Classify(cfg, Signal{Source: "github_comment", Text: "please rethink"}) != KindEscalate {
		t.Fatal("expected escalate")
	}
}
```

- [ ] **Step 2–4:** 先跑红，再实现，再跑绿。

- [ ] **Step 5: Commit**

```bash
git add internal/feedback
git commit -m "feat: add minimal feedback classifier"
```

---

### Task 6: Workspace manager

**Files:**
- Create: `internal/workspace/doc.go`
- Create: `internal/workspace/manager.go`
- Create: `internal/workspace/manager_test.go`

**Interfaces:**
- Consumes: `config.WorkspaceConfig`
- Produces: `AllocateFromLocal(src, root, branch string) (*Session, error)`；`(*Session).Snapshot() (string, error)` 返回 `git rev-parse HEAD`；`Cleanup(sess *Session, failed bool, cfg config.WorkspaceConfig, now time.Time) error`

本轮测试不访问 GitHub：用 `t.TempDir()` 初始化 git 仓库作为 src。

- [ ] **Step 1: Write the failing test**

```go
package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "ops@example.com"},
		{"git", "config", "user.name", "ops"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add: %s", out)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %s", out)
	}
	return dir
}

func TestAllocateWorktreeIndependent(t *testing.T) {
	src := initGitRepo(t)
	root := t.TempDir()
	sess, err := AllocateFromLocal(src, root, "ops-agent/issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Dir == src {
		t.Fatal("must not reuse source dir")
	}
	if _, err := os.Stat(filepath.Join(sess.Dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Dir, "x.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "x.txt")); err == nil {
		t.Fatal("source polluted")
	}
}

func TestCleanupKeepsFailed(t *testing.T) {
	src := initGitRepo(t)
	root := t.TempDir()
	sess, err := AllocateFromLocal(src, root, "ops-agent/issue-2")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.WorkspaceConfig{KeepFailed: true, CleanupAfter: "72h"}
	if err := Cleanup(sess, true, cfg, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sess.Dir); err != nil {
		t.Fatal("failed workspace should remain")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/workspace -count=1`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`AllocateFromLocal`：`os.MkdirAll(root)`，`git -C src worktree add -b <branch> <dest>`。若失败（无 worktree 支持），回退 `cp`/`git clone --local`。`Session` 含 `Dir`、`Branch`、`Source`。`Cleanup`：`failed && KeepFailed` 则 no-op；否则 `git -C src worktree remove --force dest` 或 `os.RemoveAll`。

分支名由调用方传入。编排器使用 `fmt.Sprintf("ops-agent/issue-%d", number)`。

- [ ] **Step 4: Run tests**

Run: `go test ./internal/workspace -count=1`

Expected: PASS（环境需有 `git`）

- [ ] **Step 5: Commit**

```bash
git add internal/workspace
git commit -m "feat: isolate task workspaces with git worktrees"
```

---

### Task 7: Memory 事件日志

**Files:**
- Create: `internal/memory/doc.go`
- Create: `internal/memory/store.go`
- Create: `internal/memory/store_test.go`

**Interfaces:**
- Produces: `Append(path string, ev Event) error`；`List(path, taskID string) ([]Event, error)`

```go
type Event struct {
	TaskID    string    `json:"task_id"`
	RunID     string    `json:"run_id,omitempty"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	At        time.Time `json:"at"`
}
```

JSONL 文件，每行一个 Event。

- [ ] **Step 1: Write the failing test**

```go
package memory

import (
	"path/filepath"
	"testing"
)

func TestAppendAndListFiltersByTask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := Append(path, Event{TaskID: "a", Kind: "start", Message: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, Event{TaskID: "b", Kind: "start", Message: "2"}); err != nil {
		t.Fatal(err)
	}
	got, err := List(path, "a")
	if err != nil || len(got) != 1 || got[0].Message != "1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/memory -count=1`

Expected: FAIL

- [ ] **Step 3: Write minimal JSONL Append/List**

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/memory -count=1`

Expected: PASS

- [ ] **Step 5: Commit** `feat: record task memory events`

---

### Task 8: Verifier

**Files:**
- Create: `internal/verifier/doc.go`
- Create: `internal/verifier/suite.go`
- Create: `internal/verifier/suite_test.go`

**Interfaces:**
- Consumes: `repovalidate.Validate`、`repovalidate.LoadStandard`
- Produces: `Run(ctx, Opts) Result`

```go
type Opts struct {
	RepoPath     string
	StandardsDir string
	Standard     string // 空则跳过 repo_validate
	Commands     []string
	Runner       func(ctx context.Context, dir, command string) (string, error)
}

type Result struct {
	OK      bool
	Report  string
	Command string // 第一个失败命令
}
```

默认 `Runner` 用 `exec.CommandContext("bash","-lc",command)`，`Dir=RepoPath`。测试注入假 Runner，不真跑 `go test`。

- [ ] **Step 1: Write the failing test**

```go
func TestRunFailsOnMissingStandardFile(t *testing.T) {
	dir := t.TempDir()
	res := Run(context.Background(), Opts{RepoPath: dir, StandardsDir: t.TempDir(), Standard: "missing"})
	if res.OK {
		t.Fatal("expected fail")
	}
}

func TestRunUsesInjectedRunner(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644)
	var got string
	res := Run(context.Background(), Opts{
		RepoPath: dir,
		Commands: []string{"go test ./..."},
		Runner: func(ctx context.Context, d, c string) (string, error) {
			got = c
			return "ok", nil
		},
	})
	if !res.OK || got != "go test ./..." {
		t.Fatalf("ok=%v cmd=%q report=%s", res.OK, got, res.Report)
	}
}
```

- [ ] **Step 2–4:** 红绿实现。`Standard` 非空时调用 `LoadStandard`+`Validate`；失败则 `OK=false` 并拼接 `report.Format()`。

- [ ] **Step 5: Commit** `feat: add verifier suite for format and commands`

---

### Task 9: Safety evaluator

**Files:**
- Create: `internal/safety/doc.go`
- Create: `internal/safety/evaluator.go`
- Create: `internal/safety/evaluator_test.go`

**Interfaces:**
- Produces: `Evaluate(Input) Report`

```go
type Input struct {
	DiffFiles   int
	Deleted     int
	VerifyOK    bool
	Retries     int
	MaxRetries  int
}

type Verdict string
const (
	VerdictSafeToPublish      Verdict = "safe_to_publish"
	VerdictPublishAsDraft     Verdict = "publish_as_draft"
	VerdictNeedsMoreFix       Verdict = "needs_more_fix"
	VerdictNeedsManualReview  Verdict = "needs_manual_review"
	VerdictRecommendRollback  Verdict = "recommend_rollback"
	VerdictBlock              Verdict = "block"
)
```

规则（写死，避免歧义）：

1. `Deleted >= 10` → `block`
2. `!VerifyOK && Retries >= MaxRetries && DiffFiles >= 15` → `recommend_rollback`
3. `!VerifyOK && Retries < MaxRetries` → `needs_more_fix`
4. `!VerifyOK` → `needs_manual_review`
5. `VerifyOK && DiffFiles >= 20` → `publish_as_draft`
6. `VerifyOK` → `publish_as_draft`（第一轮政策：即使低风险也只发 draft，不发 ready PR）

`safe_to_publish` 本轮不返回，避免误 merge 路径。常量保留供后续使用。

- [ ] **Step 1: Write table-driven tests for the six rules.**

- [ ] **Step 2–4:** 红绿实现。

- [ ] **Step 5: Commit** `feat: add safety verdicts for issue-to-pr`

---

### Task 10: Executor 接口与 Fake

**Files:**
- Create: `internal/executor/doc.go`
- Create: `internal/executor/types.go`
- Create: `internal/executor/fake.go`
- Create: `internal/executor/fake_test.go`

**Interfaces:**
- Produces:

```go
type Request struct {
	WorkDir string
	Repo    string
	Number  int
	Title   string
	Body    string
}

type Result struct {
	Summary string
	Changed bool
}

type Executor interface {
	Run(ctx context.Context, req Request) (Result, error)
}

type Fake struct {
	Changed bool
	Err     error
	Last    Request
}

func (f *Fake) Run(ctx context.Context, req Request) (Result, error)
```

`Fake.Run` 若 `Changed` 则在 `WorkDir` 写 `FAKE_CHANGE.md`。

- [ ] **Step 1–4:** 测试 Fake 写文件；`Err != nil` 时返回错误。

- [ ] **Step 5: Commit** `feat: add executor interface with fake`

---

### Task 11: Publisher draft PR

**Files:**
- Create: `internal/publisher/doc.go`
- Create: `internal/publisher/publisher.go`
- Create: `internal/publisher/publisher_test.go`

**Interfaces:**
- Consumes: `github.PRCreateOpts`
- Produces: `GitHubAPI` 接口；`CreateDraftPR(ctx, DraftPRInput) (string, error)`

```go
type GitHubAPI interface {
	PRCreate(ctx context.Context, opts github.PRCreateOpts) (string, error)
	IssueComment(ctx context.Context, repo string, num int, body string) error
}

type DraftPRInput struct {
	Repo        string
	Number      int
	Title       string
	Head        string
	Base        string
	Body        string
	TaskID      string
	RunID       string
}

func Footer(taskID, runID string) string
```

`CreateDraftPR`：`Draft: true`，body 末尾追加 Footer。测试用内存 FakeGitHub 记录 opts。

禁止调用 merge / close。本轮 Publisher 只暴露 `CreateDraftPR` 和 `CommentIssue`。

- [ ] **Step 1: Write failing test** 断言 `opts.Draft==true` 且 body 含 `Handled by Ops-Agent`、`Task:`、`Run:`。

- [ ] **Step 2–4:** 红绿实现。

- [ ] **Step 5: Commit** `feat: publish draft PRs with audit footer`

---

### Task 12: Orchestrator 闭环（Fake 执行器）

**Files:**
- Create: `internal/orchestrator/doc.go`
- Create: `internal/orchestrator/orchestrator.go`
- Create: `internal/orchestrator/orchestrator_test.go`
- Create: `internal/skill/issue_to_pr.go`

**Interfaces:**
- Consumes: Task 1–11 全部导出 API
- Produces: `(*Orchestrator).RunIssueToPR(ctx, RunRequest) (Outcome, error)`

```go
type RunRequest struct {
	Repo        string
	Number      int
	Title       string
	URL         string
	Body        string
	TriggerHash string
	LocalRepo   string // 测试用：跳过 gh clone，走 AllocateFromLocal
	Base        string
}

type Outcome struct {
	Task    task.Task
	Skipped bool
	PRURL   string
	Verdict safety.Verdict
}
```

编排顺序（必须按此调用，测试可用计数 Fake 检查顺序）：

1. `Ensure` Task
2. `BeginRun`；skipped 则返回
3. `Transition` → `collecting_context`
4. `AllocateFromLocal`
5. `feedback.Classify`（verifier 尚未失败时跳过，仅记录 skill 名）
6. budget `Allow`；失败 → `needs_manual_intervention`
7. `executing` + `Executor.Run`
8. `verifying` + `verifier.Run`（无 Standard、Commands 空或 `go test ./...` 若存在 go.mod）
9. 失败且 `fix_*` 且 budget 允许 → 增加 retry 再执行最多 `MaxRetries` 轮
10. `evaluating_safety`
11. `publish_as_draft` → `publishing` + `CreateDraftPR`；写 `PRURL`
12. `recommend_rollback` / `block` / `needs_manual_review` → 对应状态，不创建 PR
13. `learning`：`memory.Append`
14. `done` 或 `needs_manual_intervention`；`ClearActiveRun`

`skill/issue_to_pr.go` 导出 `const Name = "issue-to-pr"` 和 `Directive()` 字符串（中文护栏：只改相关文件、必须验证、只开 draft PR）。Executor Fake 不读它；internal-agent 后续使用。

测试用 Temp git repo + Fake executor + Fake GitHub + 空 verifier commands。

- [ ] **Step 1: Write the failing tests**

```go
func TestRunIssueToPRCreatesDraft(t *testing.T) {
	src := initGitRepoLikeWorkspaceTest(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks,
		Exec:  &executor.Fake{Changed: true},
		Publish: publisher.New(gh),
		Config:  enabledCfg(),
		LocalOnly: true,
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{
		Repo: "o/r", Number: 7, Title: "fix", TriggerHash: "t1", LocalRepo: src, Base: "main",
	})
	if err != nil || out.Skipped || out.PRURL == "" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !gh.last.Draft {
		t.Fatal("must be draft")
	}
}

func TestRunIssueToPRSkipsDuplicateTrigger(t *testing.T) {
	// 同一 TriggerHash 第二次 Skipped=true 且 PRCreate 只调用一次
}

func TestRunIssueToPRStopsOnBudget(t *testing.T) {
	// Budget.MaxSteps=0 经 Normalize 会变成 12；测试直接注入 Policy{MaxSteps:0} 的 Meter 已耗尽
}
```

把 `TestRunIssueToPRSkipsDuplicateTrigger` 与 `TestRunIssueToPRStopsOnBudget` 写完整：前者连续调用两次相同 `RunRequest`，第二次 `Skipped==true`；后者构造 `Meter` 已 `NoteStep` 超过 `MaxSteps`，期望状态 `needs_manual_intervention` 且 `PRURL==""`。`initGitRepoLikeWorkspaceTest` 复制 Task 6 的 `initGitRepo`。`enabledCfg()` 返回 `IssueToPR.Enabled=true` 的 `*config.Config`。

- [ ] **Step 2–4:** 红绿实现。GitHub 真实客户端本轮不接入测试。

- [ ] **Step 5: Commit** `feat: orchestrate issue-to-pr into draft PR`

---

### Task 13: internal-agent 适配器

**Files:**
- Create: `internal/executor/internalagent/adapter.go`
- Create: `internal/executor/internalagent/adapter_test.go`

**Interfaces:**
- Consumes: `refactor.Engine.Run` 的行为，但为可测性定义：

```go
type Engine interface {
	Run(ctx context.Context, repoPath string, item todo.Item, issue *github.Issue) (string, error)
}

type Adapter struct{ Engine Engine }

func (a Adapter) Run(ctx context.Context, req executor.Request) (executor.Result, error)
```

把 `req` 转成 `todo.Item` + `github.Issue`。`summary=="" && err==nil` 视为 `Changed=true` 若工作区 `git status --porcelain` 非空。测试用 stub Engine 返回 `"ok"`。

本轮 **不改** `refactor.Engine` 内部循环。生产接线：`internalagent.Adapter{Engine: refactor.NewEngine(cfg, gh)}` 放在 orchestrator 构造函数，仅当非测试时。

- [ ] **Step 1–4:** stub 测试。

- [ ] **Step 5: Commit** `feat: adapt refactor engine as internal executor`

---

### Task 14: Webhook 入队

**Files:**
- Create: `internal/orchestrator/enqueue.go`
- Create: `internal/orchestrator/enqueue_test.go`
- Modify: `internal/webhook/handler.go`（`Handler` 增加可选 `tasks *task.FileStore`）
- Modify: `internal/webhook/handler_test.go`（现有测试仍通过；新增 enabled 时 Ensure）
- Modify: 创建 Handler 的调用点（`internal/webhook/runtime.go` 或 `internal/tui/webhook_runtime.go`、`internal/headless/webhook.go`）——只加可选参数，nil 则旧行为。

**Interfaces:**
- Produces: `EnqueueIssue(cfg *config.Config, store *task.FileStore, repo string, iss github.Issue, triggerHash string) (created bool, t task.Task, err error)`

逻辑：

- `!cfg.Task.IssueToPR.Enabled` → 不做事，`created=false`
- 否则 `store.Ensure(...)`，`TriggerHash` 仅存到后续 Run，入队阶段不 `BeginRun`

Webhook `handleIssues` 在 `issuewatch.Enqueue` 成功后调用 `EnqueueIssue`。失败只打日志，不影响 HTTP 200（避免 GitHub 重试循环）。

- [ ] **Step 1: Write `TestEnqueueIssueDisabled` 和 `TestEnqueueIssueCreatesTask`**

- [ ] **Step 2–4:** 实现并保证 `go test ./internal/webhook ./internal/orchestrator -count=1` 通过。

- [ ] **Step 5: Commit** `feat: enqueue issue_to_pr tasks from webhooks`

---

### Task 15: 接线与文档

**Files:**
- Modify: 找到 TUI/daemon 启动 worker 的位置（`internal/tui/run.go` 或 `internal/worker`），增加可选后台 goroutine：若 `IssueToPR.Enabled`，对 `StatusCreated` 的 task 调用 `RunIssueToPR`。保持 `max_parallel=1`。
- Modify: `docs/superpowers/specs/2026-10-13-ops-agent-task-orchestrator-design.md` 不必改设计，只在 README 或 `docs/USAGE.zh-CN.md` 加一小节：如何打开 `task.issue_to_pr.enabled`。
- 不重做 TUI 面板。

接线构造：

```go
pub := publisher.New(ghClient)
orch := orchestrator.New(orchestrator.Deps{
	Tasks:     taskStore,
	Workspace: workspace.Manager{},
	Budget:    budget.FromConfig(cfg.Task.Budget),
	Exec:      internalagent.Adapter{Engine: refactor.NewEngine(cfg, ghClient)},
	Verify:    verifier.Default,
	Publish:   pub,
	Memory:    memoryPath,
	Config:    cfg,
})
```

若启动路径复杂，本任务最小集：导出 `orchestrator.New` 与 `RunIssueToPR`，用 `internal/orchestrator/wiring_test.go` 编译期断言 `Adapter` 实现 `executor.Executor`：

```go
var _ executor.Executor = internalagent.Adapter{}
```

TUI 后台循环若超过本任务合理改动面，改为在 `internal/headless` 增加 `OPS_AGENT_ISSUE_TO_PR=1` 时处理队列中一条 created task 后退出。优先走这个明确入口，避免大改 Bubble Tea。

- [ ] **Step 1: Write `TestAdapterImplementsExecutor` 和 headless 小测试（用 Fake，不调网络）。**

- [ ] **Step 2–4:** 实现 headless 分支：`OPS_AGENT_ISSUE_TO_PR=1` 时 `headless.Run` 调用 orchestrator 处理一条任务。修改 `internal/headless/env.go` 增加 `ShouldRunIssueToPR() bool`。

- [ ] **Step 5: 跑 `go test ./... -count=1`，通过后提交**

```bash
git add internal/headless internal/orchestrator docs
git commit -m "feat: enable opt-in issue-to-pr headless entry"
```

---

## Self-Review

**Spec coverage（第一轮 10 项）：**

| Spec | Task |
|------|------|
| Task 核心模型 | 1–2 |
| issue_to_pr Task | 12 |
| workspace manager | 6 |
| internal-agent 适配 | 10, 13 |
| verifier go test / repo_validate | 8 |
| safety 基础 diff | 9 |
| budget policy | 4 |
| publisher draft PR | 11 |
| memory 记录 | 7 |
| 幂等键 | 1–2, 12, 14 |
| 最小 feedback 桩 | 5 |
| 配置 | 3 |
| 入口 | 14–15 |

暂缓项均无实现任务：完整插件、Arduino matrix、完整 classifier、费用阻断、自动 Skill、merge/close、高级 TUI。

**Placeholder scan:** 无 TBD。`KeepFailed` 默认值在 Task 3 已写明。

**Type consistency:** `TypeIssueToPR`、`SourceKindIssue`、`MakeTaskKey` 格式 `repo|type|kind|number`、Verdict 字符串与 spec 一致、Publisher 只发 draft。

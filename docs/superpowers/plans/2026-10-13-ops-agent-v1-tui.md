# Ops-Agent v1 TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 v1 TUI 改成左侧可展开的任务队列、右侧可执行操作的对话，以及单独的设置页。

**Architecture:** 列表排序、展开行、按键路由、高风险确认都做成纯函数，先用测试锁住。`model.go` 只负责调用这些函数和绘制。设置页写入 `config.Save`。合并、关闭 Issue、force push 只在对话确认后调用 `github` 客户端。自动队列仍不执行这三件事。

**Tech Stack:** Go、Bubble Tea、现有 `internal/task`、`internal/config`、`internal/github`。

**Spec:** `docs/superpowers/specs/2026-10-13-ops-agent-v1-tui-design.md`

## Global Constraints

- 没选中的任务只占一行：来源仓库、编号、阶段。
- 选中任务展开四行：阶段、最近 3 行日志、停住原因、draft PR 地址。缺省写「—」。
- 未完成任务排在完成和取消之前。
- 输入框为空时 `j`/`k` 移动任务，`s` 打开设置页，`Esc` 退出程序。
- 输入框有文字时 `j`/`k`/`s` 只进入文字。
- 设置页 `Esc` 回到主界面，不退出程序。
- 设置页只有模型、webhook、代理。保存调用 `config.Save`，并作用于当前进程。
- 停止写停住原因「用户停止」。重试沿用同一条任务。跳过把状态改为取消。
- 合并、关闭 Issue、force push 先复述对象。下一句去掉首尾空白后是「确认」才执行。同时只有一条待确认动作。
- 空输入不触发动作。一句话不直接改代码，也不直接开 PR。
- 主界面不再保留 `/mode`、`/model`、`/proxy`、`/webhook` 菜单和库验收列表。

---

### Task 1: 任务行与排序

**Files:**
- Create: `internal/tui/task_rows.go`
- Create: `internal/tui/task_rows_test.go`
- Modify: `internal/tui/todo_panel.go`（`formatTodoEntry` 改为调用 `TaskRows`）

**Interfaces:**
- Consumes: `task.Task` 的 `Repo`、`SourceNumber`、`Status`、`StopReason`、`PRURL`
- Produces: `SortTasksForView(in []task.Task) []task.Task`、`TaskRows(tk task.Task, selected bool, logs []string) []string`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestSortTasksUnfinishedFirst(t *testing.T) {
	in := []task.Task{
		{SourceNumber: 1, Status: task.StatusDone},
		{SourceNumber: 2, Status: task.StatusExecuting},
		{SourceNumber: 3, Status: task.StatusCancelled},
	}
	got := SortTasksForView(in)
	if got[0].SourceNumber != 2 || got[1].SourceNumber != 1 || got[2].SourceNumber != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestSelectedTaskRows(t *testing.T) {
	rows := TaskRows(task.Task{
		Repo: "o/r", SourceNumber: 46, Status: task.StatusNeedsManualIntervention,
		StopReason: "连续 3 次测试失败",
	}, true, []string{"a", "b", "c", "d"})
	if len(rows) != 5 {
		t.Fatalf("rows=%v", rows)
	}
	if !strings.Contains(rows[0], "o/r#46") || !strings.Contains(rows[2], "b") || !strings.Contains(rows[2], "c") || strings.Contains(rows[2], "a") {
		t.Fatalf("rows=%v", rows)
	}
	if !strings.Contains(rows[3], "连续 3 次测试失败") || !strings.Contains(rows[4], "—") {
		t.Fatalf("rows=%v", rows)
	}
}

func TestUnselectedTaskIsOneLine(t *testing.T) {
	rows := TaskRows(task.Task{Repo: "o/r", SourceNumber: 45, Status: task.StatusExecuting}, false, []string{"log"})
	if len(rows) != 1 || !strings.Contains(rows[0], "o/r#45") || !strings.Contains(rows[0], string(task.StatusExecuting)) {
		t.Fatalf("rows=%v", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -count=1 -run 'TestSortTasksUnfinishedFirst|TestSelectedTaskRows|TestUnselectedTaskIsOneLine'`
Expected: FAIL，`SortTasksForView` 未定义

- [ ] **Step 3: Write minimal implementation**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func SortTasksForView(in []task.Task) []task.Task {
	out := append([]task.Task(nil), in...)
	rank := func(st task.Status) int {
		if st == task.StatusDone || st == task.StatusCancelled {
			return 1
		}
		return 0
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if rank(out[j].Status) < rank(out[i].Status) || (rank(out[j].Status) == rank(out[i].Status) && out[j].SourceNumber < out[i].SourceNumber) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TaskRows(tk task.Task, selected bool, logs []string) []string {
	head := fmt.Sprintf("%s#%d %s", tk.Repo, tk.SourceNumber, tk.Status)
	if !selected {
		return []string{head}
	}
	recent := logs
	if len(recent) > 3 {
		recent = recent[len(recent)-3:]
	}
	logText := "—"
	if len(recent) > 0 {
		logText = strings.Join(recent, " | ")
	}
	reason := tk.StopReason
	if reason == "" {
		reason = "—"
	}
	pr := tk.PRURL
	if pr == "" {
		pr = "—"
	}
	return []string{
		head,
		"阶段：" + string(tk.Status),
		"日志：" + logText,
		"原因：" + reason,
		"PR：" + pr,
	}
}
```

`renderTodoPanel` 先 `SortTasksForView(m.activeTodos())`。选中项调用 `TaskRows(..., true, logs)`，未选中调用 `TaskRows(..., false, nil)`。`logs` 先传空切片，Task 5 再接最近 3 条 memory 事件。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -count=1 -run 'TestSortTasksUnfinishedFirst|TestSelectedTaskRows|TestUnselectedTaskIsOneLine'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/task_rows.go internal/tui/task_rows_test.go internal/tui/todo_panel.go
git commit -m "$(cat <<'EOF'
Show the selected task inline and keep finished tasks last.

EOF
)"
```

### Task 2: 按键路由

**Files:**
- Create: `internal/tui/keys.go`
- Create: `internal/tui/keys_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `type KeyAction string`，常量 `KeyMoveDown`、`KeyMoveUp`、`KeyOpenSettings`、`KeyQuit`、`KeyInsert`、`KeyCloseSettings`；`RouteMainKey(inputEmpty bool, key string) KeyAction`；`RouteSettingsKey(key string) KeyAction`

- [ ] **Step 1: Write the failing test**

```go
func TestRouteMainKey(t *testing.T) {
	if RouteMainKey(true, "j") != KeyMoveDown || RouteMainKey(true, "k") != KeyMoveUp {
		t.Fatal("empty input should move")
	}
	if RouteMainKey(true, "s") != KeyOpenSettings || RouteMainKey(true, "esc") != KeyQuit {
		t.Fatal("empty input settings or quit")
	}
	if RouteMainKey(false, "j") != KeyInsert || RouteMainKey(false, "s") != KeyInsert {
		t.Fatal("typed text stays in the input")
	}
}

func TestRouteSettingsKey(t *testing.T) {
	if RouteSettingsKey("esc") != KeyCloseSettings {
		t.Fatal("settings esc closes the page")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -count=1 -run 'TestRouteMainKey|TestRouteSettingsKey'`
Expected: FAIL，`RouteMainKey` 未定义

- [ ] **Step 3: Write minimal implementation**

```go
package tui

type KeyAction string

const (
	KeyMoveDown      KeyAction = "move_down"
	KeyMoveUp        KeyAction = "move_up"
	KeyOpenSettings  KeyAction = "open_settings"
	KeyQuit          KeyAction = "quit"
	KeyInsert        KeyAction = "insert"
	KeyCloseSettings KeyAction = "close_settings"
	KeyIgnore        KeyAction = "ignore"
)

func RouteMainKey(inputEmpty bool, key string) KeyAction {
	if !inputEmpty {
		return KeyInsert
	}
	switch key {
	case "j", "down":
		return KeyMoveDown
	case "k", "up":
		return KeyMoveUp
	case "s":
		return KeyOpenSettings
	case "esc":
		return KeyQuit
	default:
		return KeyIgnore
	}
}

func RouteSettingsKey(key string) KeyAction {
	if key == "esc" {
		return KeyCloseSettings
	}
	return KeyIgnore
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -count=1 -run 'TestRouteMainKey|TestRouteSettingsKey'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/keys.go internal/tui/keys_test.go
git commit -m "$(cat <<'EOF'
Route task keys only when the chat input is empty.

EOF
)"
```

### Task 3: 对话确认状态机

**Files:**
- Create: `internal/tui/chat_action.go`
- Create: `internal/tui/chat_action_test.go`

**Interfaces:**
- Consumes: 无
- Produces:

```go
type ChatAction struct {
	Kind   string
	Repo   string
	Number int
	Ref    string
}

type ChatReply struct {
	Text    string
	Execute *ChatAction
}

func HandleChat(pending *ChatAction, line string) (next *ChatAction, reply ChatReply)
```

`Kind` 只能是 `stop`、`retry`、`skip`、`set_model`、`set_webhook`、`set_proxy`、`merge`、`close_issue`、`force_push`。

- [ ] **Step 1: Write the failing test**

```go
func TestHighRiskNeedsConfirm(t *testing.T) {
	next, reply := HandleChat(nil, "合并 o/r#9")
	if reply.Execute != nil || next == nil || next.Kind != "merge" || next.Number != 9 {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
	if !strings.Contains(reply.Text, "o/r") || !strings.Contains(reply.Text, "9") {
		t.Fatalf("text=%s", reply.Text)
	}
	next, reply = HandleChat(next, " 确认 ")
	if next != nil || reply.Execute == nil || reply.Execute.Kind != "merge" {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestOtherTextCancelsConfirm(t *testing.T) {
	pending := &ChatAction{Kind: "close_issue", Repo: "o/r", Number: 4}
	next, reply := HandleChat(pending, "先别关")
	if next != nil || reply.Execute != nil || !strings.Contains(reply.Text, "不执行") {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestNewHighRiskReplacesPending(t *testing.T) {
	pending := &ChatAction{Kind: "merge", Repo: "o/r", Number: 1}
	next, reply := HandleChat(pending, "force push o/r 分支 ops-agent/issue-2")
	if next == nil || next.Kind != "force_push" || !strings.Contains(reply.Text, "已取消") {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestStopExecutesWithoutSecondConfirm(t *testing.T) {
	next, reply := HandleChat(nil, "停止 o/r#46")
	if next != nil || reply.Execute == nil || reply.Execute.Kind != "stop" || reply.Execute.Number != 46 {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestEmptyAndCodeChangeDoNothing(t *testing.T) {
	if _, reply := HandleChat(nil, "   "); reply.Execute != nil || reply.Text != "" {
		t.Fatalf("%+v", reply)
	}
	_, reply := HandleChat(nil, "帮我改代码并开 PR")
	if reply.Execute != nil || !strings.Contains(reply.Text, "任务队列") {
		t.Fatalf("%+v", reply)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -count=1 -run 'TestHighRiskNeedsConfirm|TestOtherTextCancelsConfirm|TestNewHighRiskReplacesPending|TestStopExecutesWithoutSecondConfirm|TestEmptyAndCodeChangeDoNothing'`
Expected: FAIL，`HandleChat` 未定义

- [ ] **Step 3: Write minimal implementation**

解析规则：

- 空字符串返回空回复、不执行。
- 待确认动作存在时，输入为「确认」则返回该动作并清空 pending。其他文字返回「不执行」，pending 清空，这句话不另作命令解析。
- `合并 <repo>#<n>`、`关闭 <repo>#<n>`、`force push <repo> 分支 <ref>` 建立 pending。若已有 pending，回复包含「已取消」。
- `停止 <repo>#<n>`、`重试 <repo>#<n>`、`跳过 <repo>#<n>` 直接 `Execute`。
- `模型 <name>`、`webhook 开`、`webhook 关`、`代理 <url>`、`代理 关` 直接 `Execute`。`set_model` 的 `Ref` 是模型名，`set_proxy` 的 `Ref` 是地址或空表示关。
- 含「改代码」或「开 PR」且不是上面的命令时，回复说明自动开 draft PR 由任务队列完成。

用 `strings.TrimSpace` 和 `fmt.Sscanf(line, "合并 %s#%d", &repo, &num)` 这一类解析。force push 用 `fmt.Sscanf(line, "force push %s 分支 %s", &repo, &ref)`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -count=1 -run 'TestHighRiskNeedsConfirm|TestStopExecutesWithoutSecondConfirm|TestEmptyAndCodeChangeDoNothing'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/chat_action.go internal/tui/chat_action_test.go
git commit -m "$(cat <<'EOF'
Confirm merge, close, and force push in chat before they run.

EOF
)"
```

### Task 4: 接到界面并去掉旧菜单

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/commands.go`
- Delete: `internal/tui/mode_menu.go`、`internal/tui/mode_menu_test.go`、`internal/tui/ai_menu.go`、`internal/tui/ai_menu_test.go`、`internal/tui/proxy_menu.go`、`internal/tui/proxy_menu_test.go`、`internal/tui/webhook_menu.go`、`internal/tui/webhook_menu_test.go`、`internal/tui/webhook_conn_edit.go`
- Create: `internal/tui/settings_page.go`

**Interfaces:**
- Consumes: `RouteMainKey`、`RouteSettingsKey`、`HandleChat`、`TaskRows`、`config.Save`
- Produces: `Model.settingsOpen bool`、`Model.pending *ChatAction`。设置页渲染模型、webhook、代理三项。

- [ ] **Step 1: Write the failing test**

在 `internal/tui/commands_test.go` 新增：

```go
func TestSlashMenusAreGone(t *testing.T) {
	for _, line := range []string{"/mode", "/model", "/proxy", "/webhook"} {
		if isMenuCommand(line) {
			t.Fatalf("%s still opens a menu", line)
		}
	}
}
```

如果没有 `isMenuCommand`，测试直接读 `commands.go` 的分发函数：这些字符串不得再调用 `openModeMenu`、`openAIMenu`、`openProxyMenu`、`openWebhookMenu`。测试可以是：

```go
func TestCommandSourceDropsMenus(t *testing.T) {
	data, err := os.ReadFile("commands.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, name := range []string{"openModeMenu", "openAIMenu", "openProxyMenu", "openWebhookMenu"} {
		if strings.Contains(text, name) {
			t.Fatalf("still references %s", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -count=1 -run TestCommandSourceDropsMenus`
Expected: FAIL，文件仍包含 `openModeMenu`

- [ ] **Step 3: Write minimal implementation**

- 删除上面列出的菜单文件。
- `model.go` 的按键分支改为调用 `RouteMainKey`。`KeyMoveDown` 调用 `todoDown`，`KeyMoveUp` 调用 `todoUp`，`KeyOpenSettings` 设 `settingsOpen = true`，`KeyQuit` 返回 `tea.Quit`，`KeyInsert` 交给 textinput。
- `settingsOpen` 时 `View` 只画设置页。`Esc` 走 `RouteSettingsKey`，关闭设置页。
- 回车且输入非空时调用 `HandleChat`。`reply.Text` 追加到右侧对话。`reply.Execute` 按 Kind 调用：
  - `stop`：`tasks.CancelIssue` 后把 `StopReason` 写成「用户停止」。现有 `CancelIssue` 只改状态时，再 `Update` 写入原因。
  - `retry`：`tasks.RequeueIssue`。
  - `skip`：`tasks.CancelIssue`。
  - `set_model`、`set_webhook`、`set_proxy`：改 `m.cfg` 后 `config.Save(m.cfg)`。
  - `merge`、`close_issue`、`force_push`：只在 `HandleChat` 返回的 `Execute` 上调用。本任务若 `github.Client` 还没有对应方法，在 `internal/github/client.go` 增加 `PRMerge`、`IssueClose`、`ForcePush` 的最小 `gh` 封装，并在对话结果里写出成功或失败对象。
- 状态栏去掉可进入菜单的交互，只保留模型名、webhook 开或关、仓库、任务数。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui ./internal/github ./internal/config -count=1`
Expected: PASS

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui internal/github internal/config
git commit -m "$(cat <<'EOF'
Replace TUI menus with a settings page and confirmed chat actions.

EOF
)"
```

## Self-review

- 第 1 节的一行摘要、四行展开、最近 3 行、未完成在前，由 Task 1 覆盖。
- 第 2 节设置页和第 4 节按键，由 Task 2 与 Task 4 覆盖。
- 第 3 节确认词、替换待确认、停止重试跳过、拒绝直接改代码，由 Task 3 覆盖。
- 第 6 节自动队列不合并，本计划没有给 orchestrator 增加 merge、close 或 force push。

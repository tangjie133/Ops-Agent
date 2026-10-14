# 库问题 Arduino 测试后评论并开 draft PR Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 模型可改代码；有改动时 Arduino CLI 通过后再评论并开 draft PR；问答无改动则只评论。

**Architecture:** 执行器恢复 `edit_file`。编排器在结束后用 `git status --porcelain` 判断是否脏。脏则 `internal/arduino.Verify`（识别库并编译全部 `examples/**/*.ino`），再 `safety.Evaluate`；通过则提交推送 `ops-agent/issue-N` 或 `ops-agent/pr-N`、`CommentIssue`、`CreateDraftPR`。干净则只 `CommentIssue`。失败一律不评论、不开 PR（评论已发出后的发布失败除外）。

**Tech Stack:** Go，`arduino-cli compile --library`，现有 `gh` / `publisher`。

## Global Constraints

- 规格：`docs/superpowers/specs/2026-10-14-ops-agent-library-fix-pr-design.md`
- 不改 `README.md`、`docs/DESIGN.md`、`docs/ROADMAP.md`、`docs/superpowers/specs/2026-10-13-ops-agent-task-orchestrator-design.md` 的正文
- 实现时改 `docs/superpowers/specs/2026-10-13-ops-agent-product-roadmap-design.md` 第 2、3、8 节
- 不新建 git 分支；直接改当前工作区
- 用户未要求则不 `git commit`
- 不把 `internal/libtest` 接回编排器；Arduino 只走 `internal/arduino`
- 执行器不跑 `arduino-cli`；测试由编排器在结束后跑
- 合并 / 关闭 Issue / force push 仍只由对话「确认」触发

---

### Task 1: 执行器恢复 edit_file

**Files:**
- Modify: `internal/executor/internalagent/loop.go`
- Modify: `internal/executor/internalagent/tools.go`
- Modify: `internal/executor/internalagent/agent.go`
- Modify: `internal/executor/internalagent/agent_test.go`

**Interfaces:**
- Consumes: 现有 `runLoop`、`toolbox.run`、`executor.Result`
- Produces: `action` 增加 `Old`、`New`、`Content`。`edit_file` 改工作区文件。`Agent.Run` 在 `git status --porcelain` 非空时设 `Result.Changed=true`。`reply` 仍结束循环。

- [ ] **Step 1: Write the failing test**

在 `internal/executor/internalagent/agent_test.go` 把 `TestUnsupportedActionContinues` 换成：

```go
func TestAgentEditFileThenReply(t *testing.T) {
	dir := t.TempDir()
	run := exec.Command("git", "init", "-b", "main")
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	path := filepath.Join(dir, "src.cpp")
	if err := os.WriteFile(path, []byte("int n = 0;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := Agent{llm: &scriptLLM{replies: []string{
		`{"action":"edit_file","path":"src.cpp","old":"int n = 0;","new":"int n = 1;"}`,
		`{"action":"reply","body":"已修溢出"}`,
	}}}
	res, err := agent.Run(context.Background(), executor.Request{WorkDir: dir, Repo: "o/r", Number: 2, Title: "溢出"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "已修溢出" || !res.Changed {
		t.Fatalf("%+v", res)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "int n = 1;\n" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestEditFileRejectsContentOnExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tb := newToolbox(dir)
	_, err := tb.run(context.Background(), action{Action: "edit_file", Path: "a.txt", Content: "wipe"})
	if err == nil || !strings.Contains(err.Error(), "old/new") {
		t.Fatalf("%v", err)
	}
}
```

保留 `TestAgentRepliesAfterRead`（问答路径仍只读后 `reply`）。`scriptLLM` 不变。在该测试文件增加 `"os/exec"` 导入。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/executor/internalagent -count=1 -run 'TestAgentEditFileThenReply|TestEditFileRejectsContentOnExisting'`

Expected: FAIL（`edit_file` 仍是不支持的 action，或 `Changed` 一直为 false）

- [ ] **Step 3: Write minimal implementation**

`action` 增加：

```go
Old     string `json:"old,omitempty"`
New     string `json:"new,omitempty"`
Content string `json:"content,omitempty"`
```

`systemPrompt` 改为：

```
你是 Ops-Agent 执行器。每一轮只输出一个 JSON 对象，不要 markdown，不要其它文字。

可用 action：
1. {"action":"list_dir","path":""}
2. {"action":"search_repo","query":"关键词"}
3. {"action":"read_file","path":"相对路径","start_line":1,"end_line":200}
4. {"action":"edit_file","path":"相对路径","old":"原文片段","new":"替换片段"}  // 已有文件必须 old/new
5. {"action":"edit_file","path":"相对路径","content":"完整内容"}  // 仅当文件尚不存在
6. {"action":"reply","body":"Issue 评论正文"}

问答 Issue 不要改文件。库的缺陷只改最小相关片段。不要运行测试或 arduino-cli。不要声称已经合并或关闭 Issue。完成后必须 reply。
```

`issuePrompt` 末句改为：工作目录是该仓库副本。问答不要改文件；缺陷用 `edit_file` 后 `reply`。不要自己跑测试。

`toolbox.run` 增加 `case "edit_file": return t.editFile(a)`：

```go
func (t *toolbox) editFile(a action) (string, error) {
	full, err := t.resolve(a.Path)
	if err != nil {
		return "", err
	}
	_, statErr := os.Stat(full)
	exists := statErr == nil
	if exists {
		if a.Old == "" && a.New == "" {
			return "", fmt.Errorf("edit_file: 已有文件必须用 old/new")
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		src := string(data)
		n := strings.Count(src, a.Old)
		if n == 0 {
			return "", fmt.Errorf("edit_file: old 片段在 %s 中未找到", a.Path)
		}
		if n != 1 {
			return "", fmt.Errorf("edit_file: old 片段在 %s 中出现 %d 次", a.Path, n)
		}
		out := strings.Replace(src, a.Old, a.New, 1)
		if err := os.WriteFile(full, []byte(out), 0o644); err != nil {
			return "", err
		}
		return "updated " + a.Path, nil
	}
	if strings.TrimSpace(a.Content) == "" {
		return "", fmt.Errorf("edit_file: 新建文件须提供 content")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(a.Content), 0o644); err != nil {
		return "", err
	}
	return "created " + a.Path, nil
}
```

`Agent.Run` 成功后：

```go
changed := gitDirty(req.WorkDir)
return executor.Result{Summary: summary, Changed: changed}, nil
```

```go
func gitDirty(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) != ""
}
```

无 `.git` 时 `gitDirty` 为 false；测试仓库用 `git init`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/executor/internalagent -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

用户未要求则跳过。

---

### Task 2: Arduino CLI 库识别与示例编译

**Files:**
- Create: `internal/arduino/fqbn.go`
- Create: `internal/arduino/fqbn_test.go`
- Create: `internal/arduino/verify.go`
- Create: `internal/arduino/verify_test.go`

**Interfaces:**
- Consumes: 工作区路径、可注入的 `LookPath` 与 `RunCLI`
- Produces: `FQBN(properties string) string`；`Verify(ctx, workDir string) error`。错误文案固定：`缺少 arduino-cli`、`缺少验证`、含示例路径的编译失败、含 CLI 原文的识别/编译失败。FQBN：`architectures` 含 `esp32` → `esp32:esp32:esp32`；含 `avr`、`*`、空、其它 → `arduino:avr:uno`（同时含 esp32 与 avr 时用 esp32）。

- [ ] **Step 1: Write the failing tests**

`internal/arduino/fqbn_test.go`：

```go
package arduino

import "testing"

func TestFQBN(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"name=Lib\narchitectures=esp32\n", "esp32:esp32:esp32"},
		{"architectures=esp32,avr\n", "esp32:esp32:esp32"},
		{"architectures=avr\n", "arduino:avr:uno"},
		{"architectures=*\n", "arduino:avr:uno"},
		{"name=Lib\n", "arduino:avr:uno"},
		{"architectures=samd\n", "arduino:avr:uno"},
	}
	for _, tc := range cases {
		if got := FQBN(tc.in); got != tc.want {
			t.Fatalf("in=%q got=%s want=%s", tc.in, got, tc.want)
		}
	}
}
```

`internal/arduino/verify_test.go`：

```go
package arduino

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingCLI(t *testing.T) {
	LookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { LookPath = nil })
	err := Verify(context.Background(), t.TempDir())
	if err == nil || err.Error() != "缺少 arduino-cli" {
		t.Fatalf("%v", err)
	}
}

func TestMissingExamples(t *testing.T) {
	LookPath = func(string) (string, error) { return "/bin/arduino-cli", nil }
	t.Cleanup(func() { LookPath = nil })
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "library.properties"), []byte("architectures=avr\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Verify(context.Background(), dir)
	if err == nil || err.Error() != "缺少验证" {
		t.Fatalf("%v", err)
	}
}

func TestCompileAllExamples(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "library.properties"), []byte("architectures=esp32\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"examples/A/A.ino", "examples/nested/B/B.ino"} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("void setup(){}\nvoid loop(){}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var calls [][]string
	LookPath = func(string) (string, error) { return "/bin/arduino-cli", nil }
	RunCLI = func(ctx context.Context, dir string, args ...string) (string, error) {
		calls = append(calls, append([]string{dir}, args...))
		return "ok", nil
	}
	t.Cleanup(func() { LookPath = nil; RunCLI = nil })
	if err := Verify(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls=%d", len(calls))
	}
	for _, c := range calls {
		joined := strings.Join(c, " ")
		if !strings.Contains(joined, "compile") || !strings.Contains(joined, "--fqbn") || !strings.Contains(joined, "esp32:esp32:esp32") || !strings.Contains(joined, "--library") {
			t.Fatalf("%v", c)
		}
	}
}

func TestCompileFailureIncludesOutput(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "examples/Bad/Bad.ino")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	LookPath = func(string) (string, error) { return "/bin/arduino-cli", nil }
	RunCLI = func(ctx context.Context, dir string, args ...string) (string, error) {
		return "library not found", errors.New("exit 1")
	}
	t.Cleanup(func() { LookPath = nil; RunCLI = nil })
	err := Verify(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "examples/Bad") || !strings.Contains(err.Error(), "library not found") {
		t.Fatalf("%v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/arduino -count=1`

Expected: FAIL（包不存在）

- [ ] **Step 3: Write minimal implementation**

`internal/arduino/fqbn.go`：

```go
package arduino

import "strings"

func FQBN(properties string) string {
	arch := ""
	for _, line := range strings.Split(properties, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "architectures=") {
			arch = strings.TrimSpace(line[len("architectures="):])
			break
		}
	}
	lower := strings.ToLower(arch)
	if strings.Contains(lower, "esp32") {
		return "esp32:esp32:esp32"
	}
	return "arduino:avr:uno"
}
```

`internal/arduino/verify.go`：

```go
package arduino

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var LookPath = exec.LookPath

var RunCLI = func(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func Verify(ctx context.Context, workDir string) error {
	if _, err := LookPath("arduino-cli"); err != nil {
		return fmt.Errorf("缺少 arduino-cli")
	}
	props, _ := os.ReadFile(filepath.Join(workDir, "library.properties"))
	fqbn := FQBN(string(props))
	var sketches []string
	root := filepath.Join(workDir, "examples")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".ino") {
			rel, _ := filepath.Rel(workDir, path)
			sketches = append(sketches, filepath.ToSlash(rel))
		}
		return nil
	})
	if len(sketches) == 0 {
		return fmt.Errorf("缺少验证")
	}
	for _, rel := range sketches {
		sketchDir := filepath.Join(workDir, filepath.FromSlash(filepath.Dir(rel)))
		out, err := RunCLI(ctx, workDir, "arduino-cli", "compile", "--fqbn", fqbn, "--library", workDir, sketchDir)
		if err != nil {
			return fmt.Errorf("示例编译失败: %s\n%s", rel, out)
		}
	}
	return nil
}
```

不 upload。`RunCLI` 的 `args[0]` 必须是 `arduino-cli`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/arduino -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

用户未要求则跳过。

---

### Task 3: 编排器按是否改动分流发布

**Files:**
- Modify: `internal/task/types.go`（增加 `CommentPosted bool`）
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/orchestrator/orchestrator_test.go`
- Create: `internal/orchestrator/git.go`
- Modify: `internal/tui/task_worker.go`

**Interfaces:**
- Consumes: Task 1 的脏工作区、Task 2 的 `arduino.Verify`、`safety.Evaluate`、`publisher.CommentIssue`、`publisher.CreateDraftPR`
- Produces: `Deps.VerifyArduino func(ctx, workDir string) error`（nil 则调 `arduino.Verify`）；`Deps.CommitPush func(ctx, workDir, branch string) error`；`Deps.DefaultBranch func(ctx, repo string) (string, error)`。干净工作区：只评论，`CommentPosted=true`，`PRURL=""`。脏且 Arduino 通过且非高风险：`ops-agent/issue-N` 或来源 PR 时 `ops-agent/pr-N`，评论后 draft PR，`PRURL` 为 PR 地址。Arduino 失败、高风险、缺 CLI：`StatusFailed`，`StopReason` 为原因，`comments==0` 且不创建 PR。推送失败：不评论。评论成功但 `CreateDraftPR` 失败：`StopReason` 含 `发布失败`，`CommentPosted=true`。

- [ ] **Step 1: Write the failing tests**

在 `internal/orchestrator/orchestrator_test.go` 增加（保留 `TestRunCommentsIssue`，断言改为 `CommentPosted && PRURL==""`）：

```go
func TestDirtyWorkspaceCommentAndDraftPR(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	var pushed string
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Changed: true, Summary: "已修 read"},
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "read 溢出", "size 被忽略", nil
		},
		VerifyArduino: func(ctx context.Context, workDir string) error { return nil },
		CommitPush: func(ctx context.Context, workDir, branch string) error {
			pushed = branch
			return nil
		},
		DefaultBranch: func(ctx context.Context, repo string) (string, error) { return "main", nil },
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{
		Repo: "tangjie133/test2", Number: 2, TriggerHash: "lib", SourceKind: task.SourceKindIssue,
	})
	if err != nil || out.Skipped || gh.comments != 1 || gh.calls != 1 {
		t.Fatalf("out=%+v gh=%+v err=%v", out, gh, err)
	}
	if pushed != "ops-agent/issue-2" || gh.last.Head != "ops-agent/issue-2" || !gh.last.Draft || gh.last.Title != "read 溢出" {
		t.Fatalf("pushed=%s last=%+v", pushed, gh.last)
	}
	if !out.Task.CommentPosted || out.Task.PRURL != "https://example.test/pr/1" {
		t.Fatalf("%+v", out.Task)
	}
}

func TestArduinoFailDoesNotComment(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Changed: true, Summary: "x"},
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue:  func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) { return "t", "b", nil },
		VerifyArduino: func(ctx context.Context, workDir string) error {
			return fmt.Errorf("示例编译失败: examples/A/A.ino\nerror: overflow")
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 2, TriggerHash: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || gh.calls != 0 || !strings.Contains(out.Task.StopReason, "overflow") {
		t.Fatalf("%+v comments=%d prs=%d", out.Task, gh.comments, gh.calls)
	}
}

func TestHighRiskDoesNotPublish(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks,
		Exec: executor.ExecutorFunc(func(ctx context.Context, req executor.Request) (executor.Result, error) {
			p := filepath.Join(req.WorkDir, ".github", "workflows", "ci.yml")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Summary: "x", Changed: true}, os.WriteFile(p, []byte("on: push\n"), 0o644)
		}),
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone:      func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue:  func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) { return "t", "b", nil },
		VerifyArduino: func(ctx context.Context, workDir string) error { return nil },
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 3, TriggerHash: "risk"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || gh.calls != 0 || !strings.Contains(out.Task.StopReason, "高风险") {
		t.Fatalf("%+v", out.Task)
	}
}
```

若没有 `executor.ExecutorFunc`，在 `internal/executor/types.go` 增加：

```go
type ExecutorFunc func(ctx context.Context, req Request) (Result, error)

func (f ExecutorFunc) Run(ctx context.Context, req Request) (Result, error) { return f(ctx, req) }
```

`TestRunCommentsIssue` 改为检查 `out.Task.CommentPosted && out.Task.PRURL == ""`。`TestRetrySameTriggerCommentsAgain` 仍只计 `gh.comments`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator -count=1 -run 'TestDirtyWorkspaceCommentAndDraftPR|TestArduinoFailDoesNotComment|TestHighRiskDoesNotPublish|TestRunCommentsIssue'`

Expected: FAIL（仍无条件只评论且 `PRURL="已发出"`）

- [ ] **Step 3: Write minimal implementation**

`task.Task` 增加：

```go
CommentPosted bool `json:"comment_posted,omitempty"`
```

`Deps` 增加 `VerifyArduino`、`CommitPush`、`DefaultBranch`。

`internal/orchestrator/git.go`：

```go
func porcelain(workDir string) (deleted int, paths []string, dirty bool, err error) {
	cmd := exec.Command("git", "-C", workDir, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, nil, false, err
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		dirty = true
		code := ""
		if len(line) >= 2 {
			code = line[:2]
		}
		path := strings.TrimSpace(line[2:])
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		paths = append(paths, path)
		if strings.Contains(code, "D") {
			deleted++
		}
	}
	return deleted, paths, dirty, nil
}

func commitAndPush(ctx context.Context, workDir, branch string) error {
	cmds := [][]string{
		{"git", "-C", workDir, "checkout", "-B", branch},
		{"git", "-C", workDir, "add", "-A"},
		{"git", "-C", workDir, "-c", "user.email=ops-agent@local", "-c", "user.name=Ops-Agent", "commit", "-m", branch},
		{"git", "-C", workDir, "push", "-u", "origin", "HEAD"},
	}
	for _, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
		}
	}
	return nil
}
```

`RunIssueToPR` 在 `Exec.Run` 成功后：

1. `deleted, paths, dirty, err := porcelain(dest)`；失败则 `stop`。
2. 若不脏：现有评论路径，但 `fresh.CommentPosted = true`，不要写 `PRURL="已发出"`。
3. 若脏：`Transition` 到 `task.StatusVerifying`。`verify := o.deps.VerifyArduino`；nil 则 `arduino.Verify`。失败则 `stop(..., err.Error())`。
4. `Transition` 到 `task.StatusEvaluatingSafety`。`rep := safety.Evaluate(safety.Input{Deleted: deleted, Paths: paths, VerifyOK: true})`。`rep.Verdict != safety.VerdictPublishAsDraft` 则 `stop(..., rep.Reason)`（Reason 若不含「高风险」且是路径问题，前缀 `高风险：`）。删除文件的 Reason 已是「diff 删除了文件」，测试 `TestHighRiskDoesNotPublish` 检查「高风险」：在 orchestrator 里若 `VerdictBlock` 则 `msg := "高风险：" + rep.Reason`。
5. `branch := fmt.Sprintf("ops-agent/issue-%d", req.Number)`；若 `kind == task.SourceKindPR` 则 `ops-agent/pr-%d`。
6. `push := o.deps.CommitPush`；nil 则 `commitAndPush`。失败则 `stop`，不评论。
7. `CommentIssue`；失败则 `stop`，不开 PR。成功则先把 `CommentPosted=true` 写入 store。
8. `base := req.Base`；空则调 `DefaultBranch`，再空则 `"main"`。
9. `CreateDraftPR`，`Title: title`，`Head: branch`，`Base: base`，`Body: res.Summary`。失败则 `stop(..., "发布失败: "+err.Error())`，保留 `CommentPosted`。
10. 成功：`PRURL` 为返回 URL，`Branch` 为 branch，`StatusDone`。

`task_worker.go` 注入：

```go
VerifyArduino: arduino.Verify,
DefaultBranch: gh.RepoDefaultBranch,
```

`CommitPush` 用 `commitAndPush`，但 push 命令须走与 clone 相同的代理：在 `commitAndPush` 里对 `git push` 调用 `netproxy.ConfigureCmd(cmd, cfg.Proxy)`。为此把 `commitAndPush` 改成方法或增加 `proxy config.ProxyConfig` 参数，从 `o.deps.Config.Proxy` 读取。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator ./internal/executor/internalagent ./internal/arduino ./internal/safety -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

用户未要求则跳过。

---

### Task 4: TUI 同时显示评论与 PR

**Files:**
- Modify: `internal/tui/task_rows.go`
- Modify: `internal/tui/task_rows_test.go`

**Interfaces:**
- Consumes: `task.CommentPosted`、`task.PRURL`（真实 URL，不再用 `"已发出"` 表示评论）
- Produces: 选中任务 6 行：头、阶段、日志、原因、`评论：已发出|—`、`PR：<url>|—`

- [ ] **Step 1: Write the failing test**

替换 `TestSelectedTaskRows` / `TestSelectedTaskRowsCommentSent`：

```go
func TestSelectedTaskRows(t *testing.T) {
	rows := TaskRows(task.Task{
		Repo: "o/r", SourceNumber: 46, Status: task.StatusNeedsManualIntervention,
		StopReason: "连续 3 次测试失败",
	}, true, []string{"a", "b", "c", "d"})
	if len(rows) != 6 {
		t.Fatalf("rows=%v", rows)
	}
	if !strings.Contains(rows[4], "评论：—") || !strings.Contains(rows[5], "PR：—") {
		t.Fatalf("rows=%v", rows)
	}
}

func TestSelectedTaskRowsCommentOnly(t *testing.T) {
	rows := TaskRows(task.Task{Repo: "o/r", SourceNumber: 51, Status: task.StatusDone, CommentPosted: true}, true, nil)
	if !strings.Contains(rows[4], "评论：已发出") || !strings.Contains(rows[5], "PR：—") {
		t.Fatalf("%v", rows)
	}
}

func TestSelectedTaskRowsDraftPR(t *testing.T) {
	rows := TaskRows(task.Task{
		Repo: "o/r", SourceNumber: 2, Status: task.StatusDone,
		CommentPosted: true, PRURL: "https://github.com/o/r/pull/9",
	}, true, nil)
	if !strings.Contains(rows[4], "评论：已发出") || !strings.Contains(rows[5], "https://github.com/o/r/pull/9") {
		t.Fatalf("%v", rows)
	}
}
```

兼容：`PRURL == "已发出"` 仍显示 `评论：已发出` 且 `PR：—`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -count=1 -run 'TestSelectedTaskRows'`

Expected: FAIL（仍是 5 行，没有 PR 行）

- [ ] **Step 3: Write minimal implementation**

```go
comment := "—"
if tk.CommentPosted || tk.PRURL == "已发出" {
	comment = "已发出"
}
pr := "—"
if strings.HasPrefix(tk.PRURL, "http") {
	pr = tk.PRURL
}
return []string{head, "阶段：" + string(tk.Status), "日志：" + logText, "原因：" + reason, "评论：" + comment, "PR：" + pr}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui ./internal/orchestrator -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

用户未要求则跳过。

---

### Task 5: 路线图与规格状态

**Files:**
- Modify: `docs/superpowers/specs/2026-10-13-ops-agent-product-roadmap-design.md` 第 2、3、8 节
- Modify: `docs/superpowers/specs/2026-10-14-ops-agent-library-fix-pr-design.md` 状态改为已确认

**Interfaces:**
- Consumes: 本计划交付的双路径
- Produces: 路线图 v1 完成标准包含问答只评论、库缺陷 Arduino 通过后评论+draft PR；第 8 节流程含改代码与 Arduino CLI

- [ ] **Step 1: Edit roadmap sections 2, 3, 8**

第 2 节 v1 段改为：TUI 开着时自动入队。模型判断类型。问答只在原 Issue 或来源 PR 评论。库缺陷改代码，Arduino CLI 识别库并编译全部 `examples/` 后评论并开 draft PR。人仍负责合并、关闭 Issue、force push。细则见 `docs/superpowers/specs/2026-10-14-ops-agent-library-fix-pr-design.md`。

第 3 节完成标准增加：

- 无文件改动：只发一条评论，不开 PR。
- 有文件改动：Arduino CLI 通过且非高风险后，原 Issue 有评论且存在 draft PR。
- 缺 arduino-cli、库无法识别、示例编译失败、缺少示例、高风险改动：停住并写明原因，不评论、不开 PR。

删掉「只读调查」作为唯一路径的句子。保留人不自动合并、关闭、force push。v2 仍是规范/demo/多检查，本切片不算做完 v2。

第 8 节流程改为：

1. TUI 打开。
2. 扫描或 webhook 命中。
3. 自动入队。
4. 读上下文并克隆目标仓库。
5. 执行器调查，问答不改文件，缺陷可 `edit_file`，最后 `reply`。
6. 无改动则评论。有改动则 Arduino CLI，通过后评论并开 draft PR。
7. 克隆失败、Arduino 失败、高风险、评论/发布失败或超过时限时停住，TUI 显示原因。

规格文件顶部状态改为 `已确认`。

- [ ] **Step 2: Build and test**

Run: `go test ./internal/executor/internalagent ./internal/arduino ./internal/orchestrator ./internal/tui ./internal/safety ./internal/publisher -count=1`

Expected: PASS

Run: `go build -o /tmp/ops-agent ./cmd/ops-agent`

Expected: 成功

- [ ] **Step 3: Commit**

用户未要求则跳过。

---

## Spec coverage

| 规格 | 任务 |
|------|------|
| 问答只评论 | Task 1 + Task 3 `TestRunCommentsIssue` |
| 库缺陷改代码 | Task 1 `edit_file` |
| Arduino 三项通过条件与 FQBN | Task 2 |
| 失败不评论不开 PR | Task 3 `TestArduinoFailDoesNotComment` / `TestHighRiskDoesNotPublish` |
| 评论 + draft PR | Task 3 `TestDirtyWorkspaceCommentAndDraftPR` |
| TUI 评论行与 PR 行 | Task 4 |
| 路线图 2/3/8 | Task 5 |
| 人不自动 merge/close/force-push | 不改 `chat_action.go` |

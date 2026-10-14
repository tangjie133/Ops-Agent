# v1 Issue 评论闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 自动任务在目标仓库只读调查后，把模型自己产出的 `reply` 发到原 Issue 或来源 PR，不再开 draft PR。

**Architecture:** 编排器按任务的 `repo` 克隆临时副本，注入 Issue 正文和用户评论，调用只读执行器。执行器循环直到 `reply`、依赖失败或 `ctx` 超时。发布只走 `publisher.CommentIssue`。

**Tech Stack:** Go，`gh repo clone`，现有 `internal/ai` JSON action 循环。

## Global Constraints

- 规格：`docs/superpowers/specs/2026-10-14-ops-agent-v1-issue-comment-design.md`
- 不改 `README.md`、`docs/DESIGN.md`、`docs/ROADMAP.md`、`docs/superpowers/specs/2026-10-13-ops-agent-task-orchestrator-design.md` 的正文
- 确认后同步改路线图第 2、3、8 节里的 draft PR 完成标准
- 不强制评论；没有模型 `reply` 就不调用 GitHub 评论
- 工作区按任务 `repo` 克隆，禁止用 TUI cwd
- 用户未要求时不 `git commit`
- 不新建分支，直接改当前工作区

---

### Task 1: 执行器只读并等到 reply

**Files:**
- Modify: `internal/executor/internalagent/loop.go`
- Modify: `internal/executor/internalagent/tools.go`
- Modify: `internal/executor/internalagent/agent.go`
- Modify: `internal/executor/internalagent/agent_test.go`

**Interfaces:**
- Consumes: `LLM.ChatMessages`、`executor.Request`
- Produces: `runLoop` 在 `ctx` 取消时返回错误且不把未完成当成功；合法 `{"action":"reply","body":"..."}` 时返回 body；不再识别 `edit_file` / `done`

- [ ] **Step 1: Write the failing test**

把 `internal/executor/internalagent/agent_test.go` 的 `TestAgentEditsFile` 换成：

```go
func TestAgentRepliesAfterRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("rtc lib"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := Agent{llm: &scriptLLM{replies: []string{
		`{"action":"read_file","path":"README.md"}`,
		`{"action":"reply","body":"这是 RTC 库"}`,
	}}}
	res, err := agent.Run(context.Background(), executor.Request{WorkDir: dir, Repo: "o/r", Number: 51, Title: "介绍库", Body: "简单介绍库功能"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "这是 RTC 库" {
		t.Fatalf("%+v", res)
	}
}

func TestCancelledContextDoesNotReply(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	agent := Agent{llm: &scriptLLM{replies: []string{`{"action":"reply","body":"不该发出"}`}}}
	_, err := agent.Run(ctx, executor.Request{WorkDir: dir, Repo: "o/r", Number: 1, Title: "t"})
	if err == nil {
		t.Fatal("cancelled context must fail")
	}
}

func TestUnsupportedActionContinues(t *testing.T) {
	dir := t.TempDir()
	agent := Agent{llm: &scriptLLM{replies: []string{
		`{"action":"edit_file","path":"x","old":"a","new":"b"}`,
		`{"action":"reply","body":"只评论"}`,
	}}}
	res, err := agent.Run(context.Background(), executor.Request{WorkDir: dir, Repo: "o/r", Number: 1, Title: "t"})
	if err != nil || res.Summary != "只评论" {
		t.Fatalf("%+v %v", res, err)
	}
}
```

`scriptLLM` 在回复用尽时改为返回 `{"action":"reply","body":"done"}`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/executor/internalagent -count=1 -run 'TestAgentRepliesAfterRead|TestCancelledContextDoesNotReply|TestUnsupportedActionContinues'`

Expected: FAIL（`reply` 未作为结束动作，或仍要求 `done` / `edit_file`）

- [ ] **Step 3: Write minimal implementation**

`systemPrompt` 改为只列出 `list_dir`、`search_repo`、`read_file`、`{"action":"reply","body":"Issue 评论正文"}`。说明只读，不要改文件，不要声称已开 PR。完成后必须 `reply`。

`runLoop` 去掉 `maxSteps` 结束。循环为 `for {`，每次先 `ctx.Err()`。`act.Action == "reply"` 时返回 body。其它未知 action 的观察结果为 `不支持的 action`，继续。删除 `tools.go` 的 `edit_file` 分支。`Agent` 不再用 `maxSteps` 截断。`issuePrompt` 继续写入标题和正文。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/executor/internalagent -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

用户未要求则跳过。

---

### Task 2: 编排器克隆目标仓库并评论 Issue

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/orchestrator/orchestrator_test.go`
- Modify: `internal/orchestrator/process.go`
- Modify: `internal/tui/task_worker.go`

**Interfaces:**
- Consumes: `Deps.Clone`、`Deps.LoadIssue`、`publisher.CommentIssue`、Task 1 的 `reply` 摘要
- Produces: `Deps` 增加 `Clone func(ctx context.Context, repo, dest string) error` 和 `LoadIssue func(ctx context.Context, repo string, number int, kind task.SourceKind) (title, body string, err error)`。`RunIssueToPR` 成功时 `CommentIssue` 一次，`Task.PRURL == "已发出"`，`StatusDone`。不调用 `CreateDraftPR`、`Verify`、`safety.Evaluate`。`ProcessCreated` 不再接收 `localRepo`。

- [ ] **Step 1: Write the failing test**

替换 `internal/orchestrator/orchestrator_test.go` 里创建 draft / 验证重试 / 三次失败的测试：

```go
func TestRunCommentsIssue(t *testing.T) {
	src := initGitRepo(t)
	tasks, err := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{Summary: "介绍完成", Changed: false},
		Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone: func(ctx context.Context, repo, dest string) error {
			if repo != "tangjie133/test" {
				t.Fatalf("repo=%s", repo)
			}
			return copyGit(src, dest)
		},
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			if number != 51 || kind != task.SourceKindIssue {
				t.Fatalf("%s %d %s", repo, number, kind)
			}
			return "介绍库", "简单介绍库功能", nil
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{
		Repo: "tangjie133/test", Number: 51, TriggerHash: "t1", SourceKind: task.SourceKindIssue,
	})
	if err != nil || out.Skipped || gh.calls != 0 || gh.comments != 1 {
		t.Fatalf("out=%+v gh=%+v err=%v", out, gh, err)
	}
	if out.Task.Status != task.StatusDone || out.Task.PRURL != "已发出" || out.Task.StopReason != "" {
		t.Fatalf("%+v", out.Task)
	}
}

func TestTimeoutDoesNotComment(t *testing.T) {
	src := initGitRepo(t)
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	cfg := enabledCfg()
	cfg.Task.Budget.MaxDuration = "1ns"
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{}, Publish: publisher.New(gh), Config: cfg,
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone: func(ctx context.Context, repo, dest string) error { return copyGit(src, dest) },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "t", "b", nil
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 1, TriggerHash: "to"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || out.Task.StopReason != "超过任务时限" {
		t.Fatalf("comments=%d reason=%q", gh.comments, out.Task.StopReason)
	}
}

func TestCloneFailureStops(t *testing.T) {
	tasks, _ := task.Load(filepath.Join(t.TempDir(), "tasks.json"))
	gh := &fakeGH{}
	orch := New(Deps{
		Tasks: tasks, Exec: &executor.Fake{}, Publish: publisher.New(gh), Config: enabledCfg(),
		MemoryPath: filepath.Join(t.TempDir(), "mem.jsonl"),
		Clone: func(ctx context.Context, repo, dest string) error { return fmt.Errorf("clone denied") },
		LoadIssue: func(ctx context.Context, repo string, number int, kind task.SourceKind) (string, string, error) {
			return "t", "b", nil
		},
	})
	out, err := orch.RunIssueToPR(context.Background(), RunRequest{Repo: "o/r", Number: 2, TriggerHash: "c"})
	if err != nil {
		t.Fatal(err)
	}
	if gh.comments != 0 || !strings.Contains(out.Task.StopReason, "clone denied") {
		t.Fatalf("%+v", out.Task)
	}
}
```

保留 `TestRunIssueToPRSkipsDuplicateTrigger`，去掉 `LocalRepo` / `Verify` / `PRCreate` 断言，改为第二次 `comments` 仍为 1。`copyGit` 用 `git clone --local` 把 `src` 拷到 `dest`。`fakeGH.calls` 只在 `PRCreate` 里增加，用来保证没有开 PR。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator -count=1 -run 'TestRunCommentsIssue|TestTimeoutDoesNotComment|TestCloneFailureStops'`

Expected: FAIL（仍走 draft PR 或仍用 LocalRepo）

- [ ] **Step 3: Write minimal implementation**

`Deps` 增加 `Clone` 与 `LoadIssue`，删除 `Verify`。`RunIssueToPR`：`LoadIssue` 得到标题正文（失败则 `stop` 为 `StatusFailed`）；`os.MkdirTemp("", "ops-agent-task-")` 后 `Clone(ctx, req.Repo, dest)`，禁止使用 `req.LocalRepo`；给 `ctx` 加上 `time.ParseDuration(cfg.Task.Budget.MaxDuration)`（缺省 30m）；`Exec.Run` 的 `Body` 为 LoadIssue 正文；成功则 `Publish.CommentIssue(ctx, repo, number, summary, taskID, runID)`，`PRURL="已发出"`，`StatusDone`。`ctx.Err()` 为 `DeadlineExceeded` 时 `StopReason` 为 `超过任务时限`，不评论。去掉 `safety`、`verify`、`CreateDraftPR`。

`ProcessCreated` 改为 `ProcessCreated(ctx, orch, store)`，`RunRequest` 不带 `LocalRepo` / `Body`（由 `LoadIssue` 填充）。

`task_worker.go`：`Clone: gh.CloneRepo`；`LoadIssue` 在 `SourceKindPR` 时 `PRView`，否则 `IssueView`，用 `issuecomment.SelectRecent` 拼最近用户评论到正文后。调用 `ProcessCreated(ctx, orch, tasks)`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator ./internal/executor/internalagent ./internal/tui -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

用户未要求则跳过。

---

### Task 3: TUI 显示评论，并改路线图 v1 条文

**Files:**
- Modify: `internal/tui/task_rows.go`
- Modify: `internal/tui/task_rows_test.go`
- Modify: `docs/superpowers/specs/2026-10-13-ops-agent-product-roadmap-design.md`（仅第 2、3、8 节中与 draft PR 冲突的句子）
- Modify: `docs/superpowers/specs/2026-10-14-ops-agent-v1-issue-comment-design.md` 顶部指向本计划（若尚未指向）

**Interfaces:**
- Consumes: `Task.PRURL == "已发出"`
- Produces: 选中行第五行为 `评论：已发出` 或 `评论：—`

- [ ] **Step 1: Write the failing test**

`TestSelectedTaskRows` 末行改为包含 `评论：` 且为 `—`。新增：

```go
func TestSelectedTaskRowsCommentSent(t *testing.T) {
	rows := TaskRows(task.Task{Repo: "o/r", SourceNumber: 51, Status: task.StatusDone, PRURL: "已发出"}, true, nil)
	if !strings.Contains(rows[4], "评论：已发出") {
		t.Fatalf("%v", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -count=1 -run 'TestSelectedTaskRows'`

Expected: FAIL（仍是 `PR：`）

- [ ] **Step 3: Write minimal implementation**

`TaskRows` 第五行：`PRURL == ""` 则 `评论：—`，否则 `评论：已发出`。路线图第 2、3、8 节把“推分支并开 draft PR / 在 draft PR 上评论”改成“在原 Issue 或来源 PR 上评论”；删掉 v1 完成标准里“跑测试、高风险不改开 PR”中已不再适用的自动改代码句子，改成只读调查后评论。失败条件改为克隆失败、评论失败、超过任务时限。保留人不自动合并、关闭、force push。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...`

Expected: PASS

- [ ] **Step 5: Commit**

用户未要求则跳过。

## Self-review

- 规格第 2 节流程：Task 2 克隆与 LoadIssue，Task 3 评论行。
- 规格第 3 节执行器：Task 1。
- 规格第 3 节不强制评论、超时不评论：Task 1 取消 ctx，Task 2 `TestTimeoutDoesNotComment`。
- 规格第 4 节工作区按 `repo` 克隆：Task 2 `Clone` 断言 `tangjie133/test`。
- 规格第 5 节不再 draft PR / 测试 / 改文件：Task 1 去掉 edit，Task 2 `gh.calls == 0`。
- 规格第 6 节 `StopReason`：Task 2 失败测试。
- 规格第 7 节验收路径由上述测试覆盖；真仓库评论需跑 TUI 后人工看 GitHub。

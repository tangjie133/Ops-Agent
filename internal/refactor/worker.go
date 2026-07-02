package refactor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/investigator"
	"github.com/ZzedJay/Ops-Agent/internal/pr"
	"github.com/ZzedJay/Ops-Agent/internal/repocontext"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// Request 一次重构开 PR 任务。
type Request struct {
	Repo   string
	Number int
}

// Result Refactor Worker 执行结果。
type Result struct {
	Repo   string
	Number int
	PRURL  string
	Done   bool
	ErrMsg string
}

// Worker Issue 确认后重构并开 PR。
type Worker struct {
	cfg    *config.Config
	store  *todo.FileStore
	gh     *github.Client
	engine *Engine
	invLog investigator.Logger
}

func New(cfg *config.Config, store *todo.FileStore, gh *github.Client) *Worker {
	return &Worker{
		cfg:    cfg,
		store:  store,
		gh:     gh,
		engine: NewEngine(cfg, gh),
	}
}

func (w *Worker) SetInvestigatorLog(log investigator.Logger) {
	w.invLog = log
	if w.engine != nil {
		w.engine.SetLogger(log)
	}
}

// ProcessNext 处理队列中第一条 fix_confirmed 待办。
func (w *Worker) ProcessNext(ctx context.Context) (*Result, error) {
	if w.cfg == nil || !w.cfg.IssueAutomation.RefactorPR.Enabled {
		return &Result{}, nil
	}
	for _, item := range w.store.List() {
		if item.Status != todo.StatusFixConfirmed {
			continue
		}
		return w.Run(ctx, Request{Repo: item.Repo, Number: item.Number})
	}
	return &Result{}, nil
}

// Run 执行重构流水线（分支 → 改代码 → 测试 → push → PR）。
func (w *Worker) Run(ctx context.Context, req Request) (*Result, error) {
	res := &Result{Repo: req.Repo, Number: req.Number}
	if w.cfg == nil || !w.cfg.IssueAutomation.RefactorPR.Enabled {
		return res, fmt.Errorf("refactor_pr 未启用")
	}
	item, ok := w.store.Get(req.Repo, req.Number)
	if !ok {
		return res, fmt.Errorf("待办不存在 %s#%d", req.Repo, req.Number)
	}
	if item.Status != todo.StatusFixConfirmed {
		return res, fmt.Errorf("状态须为 fix_confirmed，当前 %s", item.Status)
	}
	if w.gh == nil {
		return res, fmt.Errorf("github client not configured")
	}

	if err := w.store.Transition(req.Repo, req.Number, todo.StatusRefactoring); err != nil {
		return res, err
	}

	fail := func(err error) (*Result, error) {
		res.ErrMsg = err.Error()
		_ = w.store.Transition(req.Repo, req.Number, todo.StatusFailed)
		return res, err
	}

	iss, err := w.gh.IssueView(ctx, req.Repo, req.Number)
	if err != nil {
		return fail(fmt.Errorf("读取 Issue: %w", err))
	}
	if !strings.EqualFold(iss.State, "OPEN") {
		return fail(fmt.Errorf("issue %s#%d 已关闭", req.Repo, req.Number))
	}

	ws := repocontext.NewWorkspace(w.cfg.AI.RepoContext, w.cfg.Proxy, w.gh)
	repoPath, err := ws.Prepare(ctx, req.Repo)
	if err != nil {
		return fail(fmt.Errorf("准备仓库: %w", err))
	}

	base, err := w.gh.RepoDefaultBranch(ctx, req.Repo)
	if err != nil {
		return fail(err)
	}
	if err := w.gh.GitCheckout(ctx, repoPath, base, w.cfg.Proxy); err != nil {
		return fail(fmt.Errorf("checkout %s: %w", base, err))
	}
	if err := w.gh.GitPull(ctx, repoPath, w.cfg.Proxy); err != nil {
		return fail(fmt.Errorf("git pull: %w", err))
	}

	branch := w.cfg.IssueAutomation.RefactorPR.BranchName(req.Number)
	if err := checkoutBranch(ctx, w.cfg.Proxy, repoPath, branch, true); err != nil {
		return fail(fmt.Errorf("创建分支 %s: %w", branch, err))
	}

	if w.engine == nil {
		w.engine = NewEngine(w.cfg, w.gh)
	}
	if w.invLog != nil {
		w.engine.SetLogger(w.invLog)
	}

	summary, err := w.engine.Run(ctx, repoPath, item, iss)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			_ = w.store.Transition(req.Repo, req.Number, todo.StatusFixConfirmed)
			return res, err
		}
		return fail(fmt.Errorf("AI 重构: %w", err))
	}

	changed, err := hasWorkingTreeChanges(ctx, w.cfg.Proxy, repoPath)
	if err != nil {
		return fail(err)
	}
	if !changed {
		return fail(fmt.Errorf("未产生代码变更（AI 摘要: %s）", truncate(summary, 200)))
	}

	testCmds := w.cfg.IssueAutomation.RefactorPR.TestCommands
	if len(testCmds) == 0 {
		testCmds = defaultTestCommands(repoPath)
	}
	if err := runTestCommands(ctx, w.cfg.Proxy, repoPath, testCmds); err != nil {
		return fail(fmt.Errorf("测试失败: %w", err))
	}
	if err := validateRepo(repoPath, w.cfg.AI.RAG); err != nil {
		return fail(err)
	}

	commitMsg := fmt.Sprintf("fix: %s (Fixes #%d)", strings.TrimSpace(item.Title), req.Number)
	if err := commitAll(ctx, w.cfg.Proxy, repoPath, commitMsg); err != nil {
		return fail(fmt.Errorf("git commit: %w", err))
	}
	if err := pushBranch(ctx, w.cfg.Proxy, repoPath, branch); err != nil {
		return fail(fmt.Errorf("git push: %w", err))
	}

	info, err := pr.GatherBranchInfoInDir(ctx, w.gh, w.cfg.Proxy, req.Repo, repoPath)
	if err != nil {
		return fail(fmt.Errorf("收集分支信息: %w", err))
	}
	draft, err := pr.GenerateDraft(ctx, w.cfg.AI, info)
	if err != nil {
		return fail(fmt.Errorf("生成 PR 描述: %w", err))
	}
	if !strings.Contains(draft.Body, fmt.Sprintf("#%d", req.Number)) {
		draft.Body = strings.TrimSpace(draft.Body) + fmt.Sprintf("\n\nFixes #%d", req.Number)
	}

	prURL, err := pr.Submit(ctx, w.gh, draft)
	if err != nil {
		return fail(fmt.Errorf("创建 PR: %w", err))
	}

	comment := fmt.Sprintf("已开 PR 修复此 Issue：%s\n\n%s", prURL, truncate(summary, 1500))
	comment = ai.FormatCommentBody(comment, w.cfg)
	if err := w.gh.IssueComment(ctx, req.Repo, req.Number, comment); err != nil {
		return fail(fmt.Errorf("Issue 回链: %w", err))
	}

	_ = w.store.SetPRURL(req.Repo, req.Number, prURL)
	if err := w.store.Transition(req.Repo, req.Number, todo.StatusPROpened); err != nil {
		return res, err
	}

	res.PRURL = prURL
	res.Done = true
	return res, nil
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

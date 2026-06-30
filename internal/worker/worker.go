package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// Analyzer 生成 Issue 回复草稿。
type Analyzer interface {
	AnalyzeIssue(ctx context.Context, repo string, num int) (string, error)
}

// Poster 发布 Issue 评论。
type Poster interface {
	IssueComment(ctx context.Context, repo string, num int, body string) error
}

type Worker struct {
	cfg      *config.Config
	store    *todo.FileStore
	analyzer Analyzer
	poster   Poster

	mu           sync.Mutex
	hourWindow   time.Time
	hourPosted   int
}

func New(cfg *config.Config, store *todo.FileStore, gh *github.Client) *Worker {
	return &Worker{
		cfg:      cfg,
		store:    store,
		analyzer: ai.NewIssueAnalyzer(cfg.AI, gh),
		poster:   gh,
	}
}

func NewWithDeps(cfg *config.Config, store *todo.FileStore, analyzer Analyzer, poster Poster) *Worker {
	return &Worker{cfg: cfg, store: store, analyzer: analyzer, poster: poster}
}

func (w *Worker) ShouldRun() bool {
	if !w.cfg.IssueAutomation.AutoAnalyze {
		return false
	}
	return w.cfg.IssueAutomation.Mode != config.ModeManual
}

// Process 处理一条 in_todo 条目（并发 1）。
func (w *Worker) Process(ctx context.Context) (*Result, error) {
	if !w.ShouldRun() {
		return &Result{}, nil
	}
	if w.analyzer == nil {
		return nil, fmt.Errorf("analyzer not configured")
	}

	for _, item := range w.store.List() {
		if item.Status != todo.StatusInTodo {
			continue
		}
		res, err := w.processItem(ctx, item)
		if err != nil {
			_ = w.store.Transition(item.Repo, item.Number, todo.StatusFailed)
			return res, err
		}
		return res, nil
	}
	return &Result{}, nil
}

type Result struct {
	Repo     string
	Number   int
	Title    string
	Draft    string
	Posted   bool
	Ready    bool
	Failed   bool
	ErrMsg   string
}

func (w *Worker) processItem(ctx context.Context, item todo.Item) (*Result, error) {
	res := &Result{Repo: item.Repo, Number: item.Number, Title: item.Title}

	if err := w.store.Transition(item.Repo, item.Number, todo.StatusAnalyzing); err != nil {
		return res, err
	}

	draft, err := w.analyzer.AnalyzeIssue(ctx, item.Repo, item.Number)
	if err != nil {
		res.Failed = true
		res.ErrMsg = err.Error()
		return res, err
	}

	if w.cfg.IssueAutomation.Mode == config.ModeFull && w.shouldAutoPost(item) {
		if err := w.postComment(ctx, item.Repo, item.Number, draft); err != nil {
			res.Failed = true
			res.ErrMsg = err.Error()
			_ = w.store.SetDraft(item.Repo, item.Number, draft)
			return res, err
		}
		if err := w.store.SetDraft(item.Repo, item.Number, draft); err != nil {
			return res, err
		}
		if err := w.store.Transition(item.Repo, item.Number, todo.StatusPosted); err != nil {
			return res, err
		}
		res.Posted = true
		res.Draft = draft
		return res, nil
	}

	if err := w.store.SetDraft(item.Repo, item.Number, draft); err != nil {
		return res, err
	}
	if err := w.store.Transition(item.Repo, item.Number, todo.StatusReady); err != nil {
		return res, err
	}
	res.Ready = true
	res.Draft = draft
	return res, nil
}

func (w *Worker) shouldAutoPost(item todo.Item) bool {
	only := w.cfg.IssueAutomation.AutoReply.OnlyLabels
	if len(only) > 0 {
		set := make(map[string]struct{}, len(item.Labels))
		for _, l := range item.Labels {
			set[l] = struct{}{}
		}
		found := false
		for _, want := range only {
			if _, ok := set[want]; ok {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	max := w.cfg.IssueAutomation.AutoReply.MaxCommentsPerHour
	if max <= 0 {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if w.hourWindow.IsZero() || now.Sub(w.hourWindow) >= time.Hour {
		w.hourWindow = now
		w.hourPosted = 0
	}
	return w.hourPosted < max
}

func (w *Worker) postComment(ctx context.Context, repo string, num int, draft string) error {
	if w.poster == nil {
		return fmt.Errorf("github poster not configured")
	}
	body := ai.FormatCommentBody(draft, w.cfg)
	if err := w.poster.IssueComment(ctx, repo, num, body); err != nil {
		return err
	}
	w.mu.Lock()
	w.hourPosted++
	w.mu.Unlock()
	return nil
}

// PostDraft 发布 semi 模式下已确认的草稿。
func (w *Worker) PostDraft(ctx context.Context, repo string, num int) error {
	item, ok := w.store.Get(repo, num)
	if !ok {
		return fmt.Errorf("todo not found")
	}
	if item.Status != todo.StatusReady {
		return fmt.Errorf("item not ready (status=%s)", item.Status)
	}
	if strings.TrimSpace(item.Draft) == "" {
		return fmt.Errorf("empty draft")
	}
	if err := w.postComment(ctx, repo, num, item.Draft); err != nil {
		return err
	}
	return w.store.Transition(repo, num, todo.StatusPosted)
}

func (w *Worker) DescribeMode() string {
	return config.ModeDescription(w.cfg.IssueAutomation.Mode)
}

func FormatResult(res *Result) string {
	if res == nil {
		return ""
	}
	ref := fmt.Sprintf("%s#%d", res.Repo, res.Number)
	switch {
	case res.Posted:
		return fmt.Sprintf("Worker: 已自动回复 %s", ref)
	case res.Ready:
		return fmt.Sprintf("Worker: 草稿就绪 %s — 选中后按 p 确认发布", ref)
	case res.Failed:
		return fmt.Sprintf("Worker: 分析失败 %s — %s", ref, res.ErrMsg)
	default:
		return ""
	}
}

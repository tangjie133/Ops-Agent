package libtest

import (
	"context"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
)

// Worker 处理 pending 验收队列。
type Worker struct {
	cfg   *config.Config
	store *FileStore
	gh    *github.Client
}

func NewWorker(cfg *config.Config, store *FileStore, gh *github.Client) *Worker {
	return &Worker{cfg: cfg, store: store, gh: gh}
}

func (w *Worker) ShouldRun() bool {
	if w.cfg == nil || w.store == nil {
		return false
	}
	w.cfg.LibTest.Normalize()
	return w.cfg.LibTest.Enabled && w.cfg.LibTest.AutoRun
}

func (w *Worker) Process(ctx context.Context) (*Item, error) {
	if !w.ShouldRun() {
		return nil, nil
	}
	for _, it := range w.store.List() {
		if it.Status != StatusPending {
			continue
		}
		return w.processOne(ctx, it)
	}
	return nil, nil
}

func (w *Worker) processOne(ctx context.Context, it Item) (*Item, error) {
	_ = w.store.Transition(it.Repo, it.Ref, StatusChecking)
	workspace, report, pass, err := RunCheck(ctx, w.gh, w.cfg, it)
	if err != nil {
		_ = w.store.SetReport(it.Repo, it.Ref, workspace, "验收失败: "+err.Error(), false)
		return nil, err
	}
	_ = w.store.SetReport(it.Repo, it.Ref, workspace, report, pass)
	out, _ := w.store.Get(it.Repo, it.Ref)
	return &out, nil
}

// RunSelected 手动对选中项执行验收。
func RunSelected(ctx context.Context, gh *github.Client, cfg *config.Config, store *FileStore, it Item) (string, bool, error) {
	_ = store.Transition(it.Repo, it.Ref, StatusChecking)
	workspace, report, pass, err := RunCheck(ctx, gh, cfg, it)
	if err != nil {
		_ = store.SetReport(it.Repo, it.Ref, workspace, "验收失败: "+err.Error(), false)
		return "", false, err
	}
	_ = store.SetReport(it.Repo, it.Ref, workspace, report, pass)
	return report, pass, nil
}

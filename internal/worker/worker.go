package worker

import (
	"context"
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

const stubDraft = "（M3 AI 分析待实现 — 当前为流水线验证占位草稿）"

type Worker struct {
	cfg   *config.Config
	store *todo.FileStore
}

func New(cfg *config.Config, store *todo.FileStore) *Worker {
	return &Worker{cfg: cfg, store: store}
}

func (w *Worker) ShouldRun() bool {
	if !w.cfg.IssueAutomation.AutoAnalyze {
		return false
	}
	return w.cfg.IssueAutomation.Mode != config.ModeManual
}

func (w *Worker) Process(ctx context.Context) (int, error) {
	_ = ctx
	if !w.ShouldRun() {
		return 0, nil
	}

	processed := 0
	for _, item := range w.store.List() {
		if item.Status != todo.StatusInTodo {
			continue
		}
		if err := w.processItem(item); err != nil {
			_ = w.store.Transition(item.Repo, item.Number, todo.StatusFailed)
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (w *Worker) processItem(item todo.Item) error {
	if err := w.store.Transition(item.Repo, item.Number, todo.StatusAnalyzing); err != nil {
		return err
	}

	draft := stubDraft
	if w.cfg.IssueAutomation.Mode == config.ModeSemi {
		draft += "\n\n[semi：M3 后将在此等待确认再回复]"
	}
	if err := w.store.SetDraft(item.Repo, item.Number, draft); err != nil {
		return err
	}
	return w.store.Transition(item.Repo, item.Number, todo.StatusReady)
}

func (w *Worker) DescribeMode() string {
	switch w.cfg.IssueAutomation.Mode {
	case config.ModeManual:
		return "manual — 扫描进待办，不自动分析"
	case config.ModeFull:
		return "full — 自动分析（M3 后将自动回复）"
	default:
		return "semi — 自动分析 → ready，M3 后确认回复"
	}
}

func FormatProcessed(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("Worker 处理了 %d 条待办 → ready", n)
}

package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/worker"
)

const manualChatSystemPrompt = `你是 Ops-Agent，GitHub 运维 TUI 助手（manual 模式）。
用户通过聊天逐步处理待办 Issue。你可以解释待办状态、Webhook、快捷键。
处理待办请让用户说：「分析/处理这条」生成草稿，「发布」发送评论，「忽略」移除待办。
回答简洁，使用中文。`

func (a *Agent) handleManualChat(ctx context.Context, line string, cx ChatContext) (string, bool, error) {
	intent := detectIntent(line)
	if intent == intentNone {
		return "", false, nil
	}

	target, err := a.resolveTarget(line, cx)
	if err != nil {
		return "", true, err
	}

	switch intent {
	case intentAnalyze:
		return a.manualAnalyze(ctx, *target)
	case intentPost:
		return a.manualPost(ctx, *target)
	case intentDismiss:
		return a.manualDismiss(*target)
	default:
		return "", false, nil
	}
}

func (a *Agent) manualAnalyze(ctx context.Context, item todo.Item) (string, bool, error) {
	ref := fmt.Sprintf("%s#%d", item.Repo, item.Number)
	if err := a.store.Transition(item.Repo, item.Number, todo.StatusAnalyzing); err != nil {
		return "", true, err
	}

	analyzer := ai.NewIssueAnalyzer(a.cfg.AI, a.cfg.Proxy, a.gh)
	if a.invLog != nil {
		analyzer.SetLogger(a.invLog)
	}
	draft, err := analyzer.AnalyzeIssue(ctx, item.Repo, item.Number)
	if err != nil {
		_ = a.store.Transition(item.Repo, item.Number, todo.StatusFailed)
		return "", true, fmt.Errorf("分析 %s 失败: %w", ref, err)
	}
	if err := a.store.SetDraft(item.Repo, item.Number, draft); err != nil {
		return "", true, err
	}
	if err := a.store.Transition(item.Repo, item.Number, todo.StatusReady); err != nil {
		return "", true, err
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("── 草稿就绪 %s ──\n\n", ref))
	b.WriteString(draft)
	b.WriteString("\n\n回复「发布」或按 p 键发送到 GitHub。")
	return b.String(), true, nil
}

func (a *Agent) manualPost(ctx context.Context, item todo.Item) (string, bool, error) {
	ref := fmt.Sprintf("%s#%d", item.Repo, item.Number)
	cur, ok := a.store.Get(item.Repo, item.Number)
	if !ok {
		return "", true, fmt.Errorf("待办不存在: %s", ref)
	}
	if cur.Status != todo.StatusReady || strings.TrimSpace(cur.Draft) == "" {
		return "", true, fmt.Errorf("%s 尚无草稿，请先发送「分析」或「处理这条」", ref)
	}
	w := worker.New(a.cfg, a.store, a.gh)
	if err := w.PostDraft(ctx, cur.Repo, cur.Number); err != nil {
		return "", true, fmt.Errorf("发布 %s 失败: %w", ref, err)
	}
	return fmt.Sprintf("已发布评论到 %s", ref), true, nil
}

func (a *Agent) manualDismiss(item todo.Item) (string, bool, error) {
	ref := fmt.Sprintf("%s#%d", item.Repo, item.Number)
	if err := a.store.Transition(item.Repo, item.Number, todo.StatusDismissed); err != nil {
		return "", true, err
	}
	return fmt.Sprintf("已忽略待办 %s", ref), true, nil
}

func manualModeHint() string {
	return "manual 模式：j/k 选中待办 → 聊天「分析/处理这条」→「发布」；或按 p 发布草稿。"
}

func semiFullAnalyzeHint() string {
	return "semi/full 模式由 Worker 自动分析；ready 后按 p 发布。切 manual 可用聊天逐步处理。"
}

func (a *Agent) selectedSummary(sel *todo.Item) string {
	if sel == nil {
		return ""
	}
	ref := fmt.Sprintf("%s#%d", sel.Repo, sel.Number)
	return fmt.Sprintf("当前选中: %s · %s · 状态 %s", ref, sel.Title, sel.Status)
}

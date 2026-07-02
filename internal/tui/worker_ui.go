package tui

// worker_ui.go — Issue Worker 的 tea.Cmd 封装与 TUI 交互（确认发布菜单等）。
import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/agent"
	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/worker"
)

const workerInterval = 8 * time.Second

type workerDoneMsg struct {
	result *worker.Result
	err    error
}

type workerTickMsg struct{}

func (m *Model) workerTickCmd() tea.Cmd {
	return tea.Tick(workerInterval, func(time.Time) tea.Msg {
		return workerTickMsg{}
	})
}

func (m *Model) refreshAIHealth() {
	health := ai.CheckHealth(m.bgCtx(), m.cfg.AI)
	m.aiOK = health.Reachable
	if !health.Reachable {
		m.aiWarn = health.Message
	}
}

func (m *Model) runWorkerCmd() tea.Cmd {
	if m.workerBusy {
		return nil
	}
	m.workerBusy = true
	invLog := m.investigatorLogFn
	ctx := m.bgCtx()
	return func() tea.Msg {
		m.refreshAIHealth()
		if !m.aiOK || m.cfg.IssueAutomation.Mode == config.ModeManual {
			return workerDoneMsg{}
		}
		if err := ctx.Err(); err != nil {
			return workerDoneMsg{err: err}
		}
		w := worker.New(m.cfg, m.store, m.gh)
		if invLog != nil {
			w.SetInvestigatorLog(invLog)
		}
		res, err := w.Process(ctx)
		return workerDoneMsg{result: res, err: err}
	}
}

func (m *Model) handleWorkerDone(msg workerDoneMsg) tea.Cmd {
	m.workerBusy = false
	_, _ = m.store.ReloadIfChanged()
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.appendLogKind(logKindWorker, "Worker: 已取消")
		} else if msg.result != nil && msg.result.ErrMsg != "" {
			m.appendLogKind(logKindError, "Worker: "+msg.result.ErrMsg)
		} else {
			m.appendLogKind(logKindError, "Worker: "+msg.err.Error())
		}
		m.ensureTodoSelection()
		m.markDirty()
		return nil
	}
	if text := worker.FormatResult(msg.result); text != "" {
		m.appendLogKind(logKindWorker, text)
		if msg.result != nil && (msg.result.Ready || msg.result.Posted) {
			m.ensureTodoSelection()
		}
	}
	m.markDirty()
	return nil
}

func (m *Model) triggerWorkerIfNeeded() tea.Cmd {
	if m.workerBusy || m.cfg.IssueAutomation.Mode == config.ModeManual || !m.aiOK || !m.hasWorkerWork() {
		return nil
	}
	return m.runWorkerCmd()
}

func (m *Model) hasWorkerWork() bool {
	for _, it := range m.store.List() {
		if it.Status == todo.StatusInTodo {
			return true
		}
		if m.cfg != nil && m.cfg.IssueAutomation.Mode == config.ModeFull &&
			it.Status == todo.StatusReady && strings.TrimSpace(it.Draft) != "" {
			return true
		}
	}
	return false
}

func (m *Model) postDraft(repo string, num int) tea.Cmd {
	ctx := m.bgCtx()
	return func() tea.Msg {
		w := worker.New(m.cfg, m.store, m.gh)
		if err := w.PostDraft(ctx, repo, num); err != nil {
			return commandDoneMsg{output: "发布失败: " + err.Error()}
		}
		return commandDoneMsg{output: fmt.Sprintf("已发布评论 %s#%d", repo, num)}
	}
}

func (m *Model) openConfirmMenu() {
	active := m.activeTodos()
	if m.todoSel < 0 || m.todoSel >= len(active) {
		m.appendOutput("请先选中一条待办")
		return
	}
	it := active[m.todoSel]
	if it.Status != todo.StatusReady || strings.TrimSpace(it.Draft) == "" {
		m.appendOutput(fmt.Sprintf("%s#%d 无草稿（状态 %s；semi 模式下 Worker 分析后为 ready）", it.Repo, it.Number, it.Status))
		return
	}
	m.confirmOpen = true
	m.confirmRepo = it.Repo
	m.confirmNum = it.Number
	m.confirmDraft = it.Draft
	m.input.Blur()
	m.layout()
}

func (m *Model) closeConfirmMenu() {
	m.confirmOpen = false
	m.confirmRepo = ""
	m.confirmNum = 0
	m.confirmDraft = ""
	m.input.Focus()
	m.layout()
}

func (m *Model) handleConfirmKey(msg string) (handled bool, cmd tea.Cmd) {
	switch msg {
	case "esc", "n":
		m.closeConfirmMenu()
		return true, nil
	case "enter", "y":
		repo, num := m.confirmRepo, m.confirmNum
		m.closeConfirmMenu()
		m.appendOutput(fmt.Sprintf("发布 %s#%d …", repo, num))
		return true, m.postDraft(repo, num)
	default:
		return false, nil
	}
}

func (m *Model) renderConfirmMenu() string {
	ref := fmt.Sprintf("%s#%d", m.confirmRepo, m.confirmNum)
	lines := []string{
		styleModeMenuTitle.Render("确认发布评论"),
		fmt.Sprintf("目标: %s", ref),
		"",
		styleModeMenuDesc.Render("草稿预览:"),
	}
	maxLen := 2000
	preview := m.confirmDraft
	if len(preview) > maxLen {
		preview = preview[:maxLen] + "\n…"
	}
	for _, line := range strings.Split(preview, "\n") {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "", styleModeMenuHint.Render("y/Enter 发布 · n/Esc 取消"))
	return m.renderMenuBox(lines)
}

func (m *Model) runAgentChat(line string) tea.Cmd {
	m.captureTodoAnchor()
	invLog := m.investigatorLogFn
	ctx := m.bgCtx()
	return func() tea.Msg {
		cx := agent.ChatContext{}
		active := m.activeTodos()
		if m.todoSel >= 0 && m.todoSel < len(active) {
			it := active[m.todoSel]
			cx.Selected = &it
		}
		a := agent.New(m.cfg, m.gh, m.store)
		if invLog != nil {
			a.SetInvestigatorLog(invLog)
		}
		out, err := a.Chat(ctx, line, cx)
		if err != nil {
			return commandDoneMsg{output: "Agent: " + err.Error()}
		}
		if out == "" {
			return commandDoneMsg{}
		}
		return commandDoneMsg{output: out}
	}
}

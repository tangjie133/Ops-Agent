package tui

// fix_ui.go — f 键确认修库（fix_confirmed），触发 Refactor Worker。

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/issuewatch"
	"github.com/ZzedJay/Ops-Agent/internal/refactor"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type refactorDoneMsg struct {
	result *refactor.Result
	err    error
}

func (m *Model) openFixConfirmMenu() {
	if !m.cfg.IssueAutomation.RefactorPR.ManualEnabled() {
		m.appendOutput("修库 PR 未启用或触发方式不含 f，请在 /mode 菜单中配置")
		return
	}
	active := m.activeTodos()
	if m.todoSel < 0 || m.todoSel >= len(active) {
		m.appendOutput("请先选中一条待办")
		return
	}
	it := active[m.todoSel]
	if !todo.CanConfirmFix(it.Status) {
		m.appendOutput(fmt.Sprintf("%s#%d 当前状态 %s 不可确认修库（需 ready/posted/failed）", it.Repo, it.Number, it.Status))
		return
	}
	m.fixConfirmOpen = true
	m.fixConfirmRepo = it.Repo
	m.fixConfirmNum = it.Number
	m.fixConfirmTitle = it.Title
	m.input.Blur()
	m.layout()
	m.markDirty()
}

func (m *Model) closeFixConfirmMenu() {
	m.fixConfirmOpen = false
	m.fixConfirmRepo = ""
	m.fixConfirmNum = 0
	m.fixConfirmTitle = ""
	m.input.Focus()
	m.layout()
	m.markDirty()
}

func (m *Model) handleFixConfirmKey(msg string) (handled bool, cmd tea.Cmd) {
	switch msg {
	case "esc", "n":
		m.closeFixConfirmMenu()
		return true, nil
	case "enter", "y":
		repo, num := m.fixConfirmRepo, m.fixConfirmNum
		m.closeFixConfirmMenu()
		m.appendOutput(fmt.Sprintf("确认修库 %s#%d …", repo, num))
		return true, m.confirmFixCmd(repo, num)
	default:
		return false, nil
	}
}

func (m *Model) confirmFixCmd(repo string, num int) tea.Cmd {
	if m.refactorBusy {
		return nil
	}
	m.refactorBusy = true
	invLog := m.investigatorLogFn
	ctx := m.bgCtx()
	return func() tea.Msg {
		res, err := issuewatch.ConfirmFixPR(m.store, repo, num)
		if err != nil {
			return refactorDoneMsg{err: err}
		}
		if !res.Confirmed {
			return refactorDoneMsg{err: fmt.Errorf("无法确认: %s", res.Reason)}
		}
		w := refactor.New(m.cfg, m.store, m.gh)
		if invLog != nil {
			w.SetInvestigatorLog(invLog)
		}
		result, err := w.Run(ctx, refactor.Request{Repo: repo, Number: num})
		return refactorDoneMsg{result: result, err: err}
	}
}

func (m *Model) handleRefactorDone(msg refactorDoneMsg) {
	if msg.err != nil {
		m.appendOutput("修库/PR: " + msg.err.Error())
		return
	}
	if msg.result != nil && msg.result.PRURL != "" {
		m.appendOutput(fmt.Sprintf("已开 PR %s#%d: %s", msg.result.Repo, msg.result.Number, msg.result.PRURL))
	}
	m.ensureTodoSelection()
	m.markDirty()
}

func (m *Model) renderFixConfirmMenu() string {
	ref := fmt.Sprintf("%s#%d", m.fixConfirmRepo, m.fixConfirmNum)
	lines := []string{
		styleModeMenuTitle.Render("确认修库并开 PR"),
		fmt.Sprintf("Issue: %s", ref),
		fmt.Sprintf("标题: %s", m.fixConfirmTitle),
		"",
		styleModeMenuDesc.Render("将标记为 fix_confirmed，随后 Refactor Worker 在分支上重构、测试并开 PR。"),
		"",
		styleModeMenuHint.Render("y/Enter 确认 · n/Esc 取消"),
	}
	return m.renderMenuBox(lines)
}

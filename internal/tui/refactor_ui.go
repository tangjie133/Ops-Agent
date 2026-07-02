package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/refactor"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func (m *Model) hasRefactorWork() bool {
	if !m.cfg.IssueAutomation.RefactorPR.Enabled {
		return false
	}
	for _, it := range m.store.List() {
		if it.Status == todo.StatusFixConfirmed {
			return true
		}
	}
	return false
}

func (m *Model) runRefactorCmd(repo string, num int) tea.Cmd {
	if m.refactorBusy {
		return nil
	}
	m.refactorBusy = true
	invLog := m.investigatorLogFn
	ctx := m.bgCtx()
	return func() tea.Msg {
		if !m.cfg.IssueAutomation.RefactorPR.Enabled {
			return refactorDoneMsg{err: fmt.Errorf("refactor_pr 未启用")}
		}
		w := refactor.New(m.cfg, m.store, m.gh)
		if invLog != nil {
			w.SetInvestigatorLog(invLog)
		}
		var result *refactor.Result
		var err error
		if repo != "" && num > 0 {
			result, err = w.Run(ctx, refactor.Request{Repo: repo, Number: num})
		} else {
			result, err = w.ProcessNext(ctx)
		}
		return refactorDoneMsg{result: result, err: err}
	}
}

func (m *Model) handleRefactorDoneMsg(msg refactorDoneMsg) tea.Cmd {
	m.refactorBusy = false
	m.handleRefactorDone(msg)
	m.ensureTodoSelection()
	m.markDirty()
	return nil
}

package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/libtest"
)

const libTestInterval = 10 * time.Second

type libTestDoneMsg struct {
	item *libtest.Item
	err  error
}

type libTestTickMsg struct{}

func (m *Model) libTestTickCmd() tea.Cmd {
	if m.cfg == nil || !m.cfg.LibTest.Enabled {
		return nil
	}
	return tea.Tick(libTestInterval, func(time.Time) tea.Msg {
		return libTestTickMsg{}
	})
}

func (m *Model) triggerLibTestIfNeeded() tea.Cmd {
	if m.libTestBusy || m.cfg == nil || !m.cfg.LibTest.Enabled || !m.cfg.LibTest.AutoRun {
		return nil
	}
	return tea.Batch(m.runLibTestCmd())
}

func (m *Model) runLibTestCmd() tea.Cmd {
	if m.libTestBusy {
		return nil
	}
	m.libTestBusy = true
	ctx := m.bgCtx()
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return libTestDoneMsg{err: err}
		}
		w := libtest.NewWorker(m.cfg, m.libTestStore, m.gh)
		item, err := w.Process(ctx)
		return libTestDoneMsg{item: item, err: err}
	}
}

func (m *Model) runSelectedLibTest() tea.Cmd {
	active := m.activeLibTests()
	if m.testSel < 0 || m.testSel >= len(active) {
		return nil
	}
	it := active[m.testSel]
	ctx := m.bgCtx()
	return func() tea.Msg {
		report, pass, err := libtest.RunSelected(ctx, m.gh, m.cfg, m.libTestStore, it)
		if err != nil {
			return commandDoneMsg{output: fmt.Sprintf("验收失败 %s@%s: %v", it.Repo, it.Ref, err)}
		}
		status := "FAIL"
		if pass {
			status = "PASS"
		}
		return commandDoneMsg{output: fmt.Sprintf("验收 %s %s@%s\n\n%s", status, it.Repo, it.Ref, truncateForDisplay(report, 8000))}
	}
}

func (m *Model) handleLibTestDone(msg libTestDoneMsg) tea.Cmd {
	m.libTestBusy = false
	if msg.err != nil && msg.item != nil {
		m.appendLogKind(logKindError, fmt.Sprintf("验收: %s@%s %v", msg.item.Repo, msg.item.Ref, msg.err))
	}
	if msg.item != nil {
		st := "完成"
		if msg.item.Status == libtest.StatusFail {
			st = "未通过"
		} else if msg.item.Status == libtest.StatusPass {
			st = "通过"
		}
		m.appendLogKind(logKindWorker, fmt.Sprintf("验收: %s %s@%s", st, msg.item.Repo, msg.item.Ref))
	}
	m.ensureTestSelection()
	return m.libTestTickCmd()
}

func (m *Model) handleLibTestTick() tea.Cmd {
	cmds := []tea.Cmd{m.libTestTickCmd()}
	if m.cfg != nil && m.cfg.LibTest.Enabled && m.cfg.LibTest.AutoRun && !m.libTestBusy {
		cmds = append(cmds, m.runLibTestCmd())
	}
	return tea.Batch(cmds...)
}

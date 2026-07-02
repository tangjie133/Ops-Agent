package tui

// pr_ui.go — /describe、/pr：AI 生成 PR 草稿并预览确认后 gh pr create。

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/pr"
)

type prDescribeDoneMsg struct {
	draft *pr.Draft
	err   error
}

func isPRDescribeCommand(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	switch line {
	case "/describe", "/pr", "/pr create", "/pr describe":
		return true
	default:
		return false
	}
}

func isPRDescribeIntent(line string) bool {
	if isPRDescribeCommand(line) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(line))
	if strings.Contains(lower, "pull request") || strings.Contains(lower, "pr 描述") || strings.Contains(lower, "pr描述") {
		return true
	}
	switch {
	case strings.Contains(lower, "创建") && (strings.Contains(lower, " pr") || strings.HasSuffix(lower, "pr") || strings.Contains(lower, "pull request")):
		return true
	case strings.Contains(lower, "开") && strings.Contains(lower, "pr"):
		return true
	case strings.Contains(lower, "create") && strings.Contains(lower, "pr"):
		return true
	case strings.Contains(lower, "写") && strings.Contains(lower, "pr"):
		return true
	default:
		return false
	}
}

func (m *Model) runDescribeCmd() tea.Cmd {
	if !m.aiOK {
		return func() tea.Msg {
			return prDescribeDoneMsg{err: fmt.Errorf("AI 未就绪，请先配置 /model 并启动 llama-server")}
		}
	}
	ctx := m.bgCtx()
	gh := m.gh
	cfg := m.cfg
	return func() tea.Msg {
		info, err := pr.GatherBranchInfo(ctx, gh, cfg.Proxy, "")
		if err != nil {
			return prDescribeDoneMsg{err: err}
		}
		draft, err := pr.GenerateDraft(ctx, cfg.AI, info)
		if err != nil {
			return prDescribeDoneMsg{err: err}
		}
		return prDescribeDoneMsg{draft: draft}
	}
}

func (m *Model) openPRConfirmMenu(d *pr.Draft) {
	m.prConfirmOpen = true
	m.prConfirmRepo = d.Repo
	m.prConfirmTitle = d.Title
	m.prConfirmBody = d.Body
	m.prConfirmBase = d.BaseBranch
	m.prConfirmHead = d.HeadBranch
	m.prConfirmEditNum = d.ExistingPR
	m.prConfirmExistingURL = d.ExistingURL
	m.input.Blur()
	m.layout()
	m.markDirty()
}

func (m *Model) closePRConfirmMenu() {
	m.prConfirmOpen = false
	m.prConfirmRepo = ""
	m.prConfirmTitle = ""
	m.prConfirmBody = ""
	m.prConfirmBase = ""
	m.prConfirmHead = ""
	m.prConfirmEditNum = 0
	m.prConfirmExistingURL = ""
	m.input.Focus()
	m.layout()
	m.markDirty()
}

func (m *Model) handlePRConfirmKey(msg string) (handled bool, cmd tea.Cmd) {
	switch msg {
	case "esc", "n":
		m.closePRConfirmMenu()
		return true, nil
	case "enter", "y":
		d := &pr.Draft{
			Repo:        m.prConfirmRepo,
			Title:       m.prConfirmTitle,
			Body:        m.prConfirmBody,
			BaseBranch:  m.prConfirmBase,
			HeadBranch:  m.prConfirmHead,
			ExistingPR:  m.prConfirmEditNum,
			ExistingURL: m.prConfirmExistingURL,
		}
		m.closePRConfirmMenu()
		if d.ExistingPR > 0 {
			m.appendOutput(fmt.Sprintf("更新 PR #%d …", d.ExistingPR))
		} else {
			m.appendOutput(fmt.Sprintf("创建 PR %s (%s → %s) …", d.Repo, d.HeadBranch, d.BaseBranch))
		}
		return true, m.submitPRCmd(d)
	default:
		return false, nil
	}
}

func (m *Model) submitPRCmd(d *pr.Draft) tea.Cmd {
	ctx := m.bgCtx()
	gh := m.gh
	return func() tea.Msg {
		url, err := pr.Submit(ctx, gh, d)
		if err != nil {
			return commandDoneMsg{output: "PR 操作失败: " + err.Error()}
		}
		if d.ExistingPR > 0 {
			return commandDoneMsg{output: fmt.Sprintf("已更新 PR #%d\n%s", d.ExistingPR, url)}
		}
		return commandDoneMsg{output: "已创建 PR:\n" + url}
	}
}

func (m *Model) renderPRConfirmMenu() string {
	action := "创建 Pull Request"
	if m.prConfirmEditNum > 0 {
		action = fmt.Sprintf("更新 PR #%d", m.prConfirmEditNum)
	}
	lines := []string{
		styleModeMenuTitle.Render("确认 " + action),
		fmt.Sprintf("仓库: %s", m.prConfirmRepo),
		fmt.Sprintf("分支: %s → %s", m.prConfirmHead, m.prConfirmBase),
		"",
		styleModeMenuDesc.Render("标题:"),
		"  " + m.prConfirmTitle,
		"",
		styleModeMenuDesc.Render("正文预览:"),
	}
	maxLen := 2400
	preview := m.prConfirmBody
	if len(preview) > maxLen {
		preview = preview[:maxLen] + "\n…"
	}
	for _, line := range strings.Split(preview, "\n") {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "", styleModeMenuHint.Render("y/Enter 确认 · n/Esc 取消"))
	return m.renderMenuBox(lines)
}

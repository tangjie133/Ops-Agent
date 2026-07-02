package tui

// model.go — Bubble Tea 根模型：输入、输出 viewport、菜单状态与 Update/View 主逻辑。

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type spinnerTickMsg struct{} // deprecated: spinner 由 refresh tick 驱动

type startupDoneMsg struct {
	ghOK   bool
	ghWarn string
	aiOK   bool
	aiWarn string
	repo   string
}

type commandDoneMsg struct {
	output string
}

type webhookStartedMsg struct {
	err error
}

// Model 是 Bubble Tea 的根模型，持有 UI 状态并处理所有 tea.Msg。
type Model struct {
	cfg            *config.Config
	gh             *github.Client
	store          *todo.FileStore
	libTestStore   *libtest.FileStore
	whRuntime      *WebhookRuntime
	input          textinput.Model
	outputViewport viewport.Model
	outputContent  string
	width          int
	height         int

	ghOK   bool // GitHub CLI 是否就绪
	ghWarn string
	aiOK   bool // llama-server / AI 是否可达
	aiWarn string
	repo   string

	todoSel int
	testSel int
	todoAnchorRepo string // 待办选中锚点（列表重排后仍指向同一 Issue）
	todoAnchorNum  int
	testAnchorRepo string // 验收选中锚点
	testAnchorRef  string
	leftFocus leftPanelFocus // 左栏焦点：待办 或 验收
	spinnerFrame int
	spinnerActive bool
	ready   bool

	modeMenuOpen bool
	modeMenuSel  int

	acceptMenuOpen bool
	acceptMenuSel  int

	webhookMenuOpen  bool
	webhookMenuLevel int
	webhookMenuSel   int
	webhookEditField int

	aiMenuOpen  bool
	aiMenuLevel int
	aiMenuSel   int
	aiEditField int
	aiInput     textinput.Model

	proxyMenuOpen  bool
	proxyMenuLevel int
	proxyMenuSel   int
	proxyEditField int
	proxyInput     textinput.Model

	menuNotice       string
	connInput        textinput.Model

	completions []Completion
	completeIdx int

	confirmOpen  bool
	confirmRepo  string
	confirmNum   int
	confirmDraft string

	invLogSink *investigatorLogSink
	invStatus  string

	workerBusy  bool // 主线程设置，防止 Worker 重入
	libTestBusy bool

	runCtx context.Context // 退出时 cancel，终止后台 tea.Cmd

	investigatorLogFn func(string)

	viewCache viewCacheState // View 输出缓存，减轻 lipgloss 重绘
}

// NewModel 构造 TUI 初始状态（尺寸在 WindowSizeMsg 后 layout）。
func NewModel(cfg *config.Config, store *todo.FileStore, libTestStore *libtest.FileStore, wh *WebhookRuntime) Model {
	ti := textinput.New()
	ti.Placeholder = "ask a question, or describe a task  (/help)"
	ti.Focus()
	ti.CharLimit = 2048
	ti.Width = 60
	_ = ti.Cursor.SetMode(cursor.CursorStatic)

	m := Model{
		cfg:            cfg,
		gh:             github.NewClientWithProxy(cfg.Proxy),
		store:          store,
		libTestStore:   libTestStore,
		whRuntime:      wh,
		input:          ti,
		outputViewport: viewport.New(60, 8),
	}
	m.syncOutputViewport(true)
	m.ensureTodoSelection()
	m.ensureTestSelection()
	return m
}

func (m *Model) bgCtx() context.Context {
	if m.runCtx != nil {
		return m.runCtx
	}
	return context.Background()
}

func (m *Model) Init() tea.Cmd {
	// 并行启动：自检、Webhook、UI 轮询 tick、Worker/LibTest/诊断 tick
	cmds := []tea.Cmd{
		m.runStartup(),
		m.startWebhookCmd(),
		m.refreshTickCmd(),
		m.workerTickCmd(),
	}
	if m.cfg != nil && m.cfg.LibTest.Enabled {
		cmds = append(cmds, m.libTestTickCmd())
	}
	if diagEnabled() {
		cmds = append(cmds, m.diagTickCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) runStartup() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		msg := startupDoneMsg{}

		if !m.gh.Available() {
			msg.ghOK = false
			msg.ghWarn = "gh 未安装或不在 PATH"
			return msg
		}

		auth, _ := m.gh.AuthStatus(ctx)
		if !auth.LoggedIn {
			msg.ghOK = false
			msg.ghWarn = "gh 未登录 — 运行 gh auth login"
		} else {
			msg.ghOK = true
			repo, err := m.gh.RepoFromCwd(ctx)
			if err != nil {
				msg.repo = "—"
				msg.ghWarn = fmt.Sprintf("无法解析当前仓库: %v", err)
			} else {
				msg.repo = repo
			}
		}

		health := ai.CheckHealth(ctx, m.cfg.AI)
		msg.aiOK = health.Reachable
		if !health.Reachable {
			msg.aiWarn = health.Message
		}

		return msg
	}
}

func (m *Model) startWebhookCmd() tea.Cmd {
	return func() tea.Msg {
		if m.whRuntime == nil {
			return webhookStartedMsg{}
		}
		return webhookStartedMsg{err: m.whRuntime.Start()}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	start := time.Now()
	msgType := diagMsgType(msg)
	defer func() { recordUpdate(msgType, time.Since(start)) }()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmOpen {
			handled, postCmd := m.handleConfirmKey(msg.String())
			if handled {
				return m, postCmd
			}
		}
		if m.aiMenuOpen && m.aiEditField >= 0 {
			return m.handleAIConnEdit(msg)
		}
		if m.aiMenuOpen {
			if m.handleAIMenuKey(msg.String()) {
				return m, textinput.Blink
			}
			return m, nil
		}
		if m.proxyMenuOpen && m.proxyEditField >= 0 {
			return m.handleProxyConnEdit(msg)
		}
		if m.proxyMenuOpen {
			if m.handleProxyMenuKey(msg.String()) {
				return m, textinput.Blink
			}
			return m, nil
		}
		if m.webhookMenuOpen && m.webhookEditField >= 0 {
			return m.handleWebhookConnEdit(msg)
		}
		if m.webhookMenuOpen {
			if m.handleWebhookMenuKey(msg.String()) {
				return m, textinput.Blink
			}
			return m, nil
		}
		if m.modeMenuOpen {
			if m.handleModeMenuKey(msg.String()) {
				return m, textinput.Blink
			}
			return m, nil
		}
		if m.acceptMenuOpen {
			if handled, cmd := m.handleAcceptMenuKey(msg.String()); handled {
				return m, cmd
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+y":
			if n, err := m.copyLogsToClipboard(); err != nil {
				m.appendOutput("复制日志失败: " + err.Error())
			} else {
				m.appendOutput(fmt.Sprintf("已复制 %d 行日志到剪贴板（另存于 %s）", n, config.LogFilePath()))
			}
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.confirmOpen {
				m.closeConfirmMenu()
				return m, nil
			}
			return m, tea.Quit
		case "j":
			if m.input.Value() == "" {
				if m.leftFocus == focusTest {
					m.testDown()
				} else {
					m.todoDown()
				}
				return m, nil
			}
		case "k":
			if m.input.Value() == "" {
				if m.leftFocus == focusTest {
					m.testUp()
				} else {
					m.todoUp()
				}
				return m, nil
			}
		case "[":
			if m.input.Value() == "" {
				m.leftFocus = focusTodo
				m.markDirty()
				return m, nil
			}
		case "]":
			if m.input.Value() == "" {
				m.leftFocus = focusTest
				m.markDirty()
				return m, nil
			}
		case "d":
			if m.input.Value() == "" {
				if m.leftFocus == focusTest {
					m.dismissSelectedTest()
				} else {
					m.dismissSelectedTodo()
				}
				return m, nil
			}
		case "i":
			if m.input.Value() == "" && m.leftFocus == focusTodo {
				return m, m.focusSelectedTodo()
			}
		case "v":
			if m.input.Value() == "" && m.leftFocus == focusTest {
				m.showSelectedTestReport()
				return m, nil
			}
		case "p":
			if m.input.Value() == "" && m.leftFocus == focusTodo {
				m.openConfirmMenu()
				return m, nil
			}
		case "tab":
			if m.applyCompletionTab() {
				return m, nil
			}
		case "right":
			if m.applyCompletionGhost() {
				return m, nil
			}
		case "enter":
			if m.input.Value() == "" {
				if m.leftFocus == focusTest {
					return m, m.runSelectedLibTest()
				}
				return m, nil
			}
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.resetCompletions()
			if isOutputClearCommand(line) {
				m.clearOutput()
				return m, nil
			}
			if isLogsCopyCommand(line) {
				if n, err := m.copyLogsToClipboard(); err != nil {
					m.appendOutput("复制日志失败: " + err.Error())
				} else {
					m.appendOutput(fmt.Sprintf("已复制 %d 行日志到剪贴板\n文件: %s", n, config.LogFilePath()))
				}
				return m, nil
			}
			m.appendOutput("> " + line)
			if isModeMenuCommand(line) {
				m.openModeMenu()
				return m, textinput.Blink
			}
			if isWebhookMenuCommand(line) {
				m.openWebhookMenu()
				return m, textinput.Blink
			}
			if isAIMenuCommand(line) {
				m.openAIMenu()
				return m, textinput.Blink
			}
			if isProxyMenuCommand(line) {
				m.openProxyMenu()
				return m, textinput.Blink
			}
			if isAcceptMenuCommand(line) {
				m.openAcceptMenu()
				return m, nil
			}
			if !strings.HasPrefix(line, "/") {
				return m, m.runAgentChat(line)
			}
			return m, m.runCommand(line)
		}

	case startupDoneMsg:
		m.ghOK = msg.ghOK
		m.ghWarn = msg.ghWarn
		m.aiOK = msg.aiOK
		m.aiWarn = msg.aiWarn
		m.repo = msg.repo
		m.ready = true
		m.markDirty()

		var lines []string
		if !m.ghOK {
			lines = append(lines, styleStatusErr.Render("✗ "+m.ghWarn))
		} else {
			lines = append(lines, styleStatusOK.Render("✓ GitHub CLI 就绪"))
		}
		if !m.aiOK {
			lines = append(lines, styleStatusWarn.Render("⚠ "+m.aiWarn+"（semi/full 需 llama-server）"))
		} else {
			lines = append(lines, styleStatusOK.Render("✓ llama-server 可达"))
		}
		m.appendOutput(strings.Join(lines, "\n"))
		if m.cfg.Webhook.Enabled && m.whRuntime != nil {
			var wh []string
			wh = append(wh, "Webhook: "+m.whRuntime.ListenURL())
			if m.cfg.Webhook.Tunnel.Smee.Enabled {
				wh = append(wh, "Smee: "+m.whRuntime.SmeeSummary())
			}
			if payload := m.whRuntime.PayloadURL(); payload != "" {
				wh = append(wh, "Payload: "+payload)
			}
			m.appendOutput(strings.Join(wh, " · "))
		}
		m.appendOutput("输入 /help 查看命令 · /webhook /accept /model /proxy 配置")
		m.appendOutput("后台日志: " + config.LogFilePath() + "（Ctrl+Y 复制 · tail -f 查看）")
		m.appendOutput("强制退出: 连按两次 Ctrl+C，或另开终端 make kill")
		if diagEnabled() {
			m.appendOutput("诊断日志: " + config.DiagLogFilePath() + "（卡顿时 tail -f）")
		}
		return m, nil

	case webhookStartedMsg:
		if msg.err != nil {
			m.appendOutput(styleStatusErr.Render("✗ Webhook 启动失败: " + msg.err.Error()))
		}
		return m, nil

	case refreshTickMsg:
		return m, m.handleRefreshTick()

	case workerTickMsg:
		cmds := []tea.Cmd{m.workerTickCmd()}
		if !m.workerBusy && m.cfg.IssueAutomation.Mode != config.ModeManual && m.aiOK && m.cfg.IssueAutomation.AutoAnalyze && m.hasWorkerWork() {
			cmds = append(cmds, m.runWorkerCmd())
		}
		return m, tea.Batch(cmds...)

	case libTestTickMsg:
		return m, m.handleLibTestTick()

	case diagTickMsg:
		return m, m.handleDiagTick()

	case libTestDoneMsg:
		return m, m.handleLibTestDone(msg)

	case workerDoneMsg:
		cmd := m.handleWorkerDone(msg)
		m.clearInvStatus()
		return m, cmd

	case LogLineMsg:
		if msg.Line != "" {
			bgLog.append(logKindWebhook, msg.Line)
		}
		return m, nil

	case invStatusMsg:
		return m, nil

	case commandDoneMsg:
		if msg.output != "" {
			m.appendOutput(truncateForDisplay(msg.output, 12000))
		}
		m.ensureTodoSelection()
		m.markDirty()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		m.invalidateViewCache()
		return m, nil

	case tea.MouseMsg:
		// 仅处理滚轮；忽略 motion/click，避免 cell motion 模式下的消息洪峰。
		if !tea.MouseEvent(msg).IsWheel() {
			return m, nil
		}
		if m.handleMouseScroll(msg) {
			m.markDirty()
			return m, nil
		}
		return m, nil
	}

	// 仅键盘输入交给 textinput；其它内部消息（BlinkMsg 等）若传入 Update
	// 会触发无意义的 View 重绘，长时间运行后会造成事件洪峰。
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.modeMenuOpen || m.webhookMenuOpen || m.aiMenuOpen || m.proxyMenuOpen || m.confirmOpen || m.acceptMenuOpen {
		return m, nil
	}
	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	if m.input.Value() != prev {
		m.resetCompletions()
	}
	m.refreshCompletions()
	m.markDirty()
	return m, cmd
}

func (m *Model) runCommand(line string) tea.Cmd {
	ctx := m.bgCtx()
	return func() tea.Msg {
		out := runCommand(ctx, m.cfg, m.gh, m.store, line)
		return commandDoneMsg{output: out}
	}
}

func (m *Model) clearOutput() {
	m.outputContent = ""
	m.syncOutputViewport(true)
	m.markDirty()
}

func (m *Model) appendOutput(s string) {
	atBottom := m.outputContent == "" || m.outputViewport.AtBottom()
	if m.outputContent != "" {
		m.outputContent += "\n"
	}
	m.outputContent += s
	m.outputContent = trimOutputContent(m.outputContent)
	m.syncOutputViewport(atBottom)
	m.markDirty()
}

func (m *Model) syncOutputViewport(stickBottom bool) {
	content := m.outputContent
	if content == "" {
		content = outputPlaceholder
	}
	m.outputViewport.SetContent(content)
	if stickBottom {
		m.outputViewport.GotoBottom()
	}
}

func (m *Model) handleMouseScroll(msg tea.MouseMsg) bool {
	if !m.isInOutputArea(msg.Y) {
		return false
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.outputViewport.LineUp(3)
		return true
	case tea.MouseButtonWheelDown:
		m.outputViewport.LineDown(3)
		return true
	}
	return false
}

func (m *Model) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	outW := m.outputWidth()
	if outW < 10 {
		outW = 10
	}

	m.outputViewport.Width = outW
	m.outputViewport.Height = m.chatHeight()
	m.input.Width = max(20, m.width-6)
	m.syncOutputViewport(m.outputViewport.AtBottom())
	m.markDirty()
}

func (m *Model) outputWidth() int {
	todoW := m.todoPanelWidth()
	outW := m.width - todoW - 4
	if outW < 20 {
		return m.width - 2
	}
	return outW
}

func (m *Model) renderHeader() string {
	return m.renderHeaderCached()
}

func (m *Model) todoPanelWidth() int {
	w := min(44, m.width*2/5)
	if w < 32 {
		w = 32
	}
	return w
}

func (m *Model) renderTodoPanel() string {
	active := m.activeTodos()
	if len(active) == 0 {
		return styleTodoItem.Render("  (无)")
	}
	m.ensureTodoSelection()

	maxLines := m.bodyHeight() - 1
	if maxLines < 1 {
		maxLines = 5
	}
	lineWidth := m.todoPanelWidth() - 2
	start := panelScrollStart(m.todoSel, len(active), maxLines)
	var lines []string
	if start > 0 {
		lines = append(lines, styleTodoItem.Render(fmt.Sprintf("  …+%d ↑", start)))
	}
	for i := start; i < len(active); i++ {
		it := active[i]
		entry := formatTodoEntry(it, lineWidth, i == m.todoSel, m.spinnerFrame)
		if len(lines)+len(entry) > maxLines {
			remaining := len(active) - i
			if remaining > 0 {
				lines = append(lines, styleTodoItem.Render(fmt.Sprintf("  …+%d", remaining)))
			}
			break
		}
		for j, line := range entry {
			style := styleTodoItem
			if i == m.todoSel && j == 0 {
				style = styleTodoSelected
			} else if j == 0 && it.Status == todo.StatusAnalyzing {
				style = styleTodoAnalyzing
			}
			lines = append(lines, style.Render(line))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderBody() string {
	todoW := m.todoPanelWidth()
	outW := m.outputWidth()
	chatView := lipgloss.NewStyle().Width(outW).Height(m.chatHeight()).Render(m.outputViewport.View())

	var right strings.Builder
	right.WriteString(chatView)

	if outW >= 20 && m.width > todoW+4 {
		left := m.renderLeftColumn()
		return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right.String())
	}
	return right.String()
}

func (m *Model) renderFooter() string {
	var b strings.Builder
	b.WriteString("\n")

	if m.webhookMenuOpen {
		b.WriteString(m.renderWebhookMenu())
		b.WriteString(m.renderWebhookConnEditBar())
		b.WriteString("\n")
		return b.String()
	}

	if m.aiMenuOpen {
		b.WriteString(m.renderAIMenu())
		b.WriteString(m.renderAIConnEditBar())
		b.WriteString("\n")
		return b.String()
	}

	if m.proxyMenuOpen {
		b.WriteString(m.renderProxyMenu())
		b.WriteString(m.renderProxyConnEditBar())
		b.WriteString("\n")
		return b.String()
	}

	if m.modeMenuOpen {
		b.WriteString(m.renderModeMenu())
		b.WriteString("\n")
		return b.String()
	}

	if m.acceptMenuOpen {
		b.WriteString(m.renderAcceptMenu())
		b.WriteString("\n")
		return b.String()
	}

	if m.confirmOpen {
		b.WriteString(m.renderConfirmMenu())
		b.WriteString("\n")
		return b.String()
	}

	line := m.input.View()
	if ghost := ghostSuffix(m.input.Value(), m.completions); ghost != "" {
		line += styleCompleteGhost.Render(ghost)
	}
	b.WriteString(line)
	b.WriteString("\n")

	if bar := m.renderCompletionBar(); bar != "" {
		b.WriteString(bar)
		b.WriteString("\n")
	}

	b.WriteString(styleHelp.Render("[/] 待办/验收 · j/k 移动 · Ctrl+Y 复制日志"))
	return b.String()
}

func (m *Model) renderCompletionBar() string {
	if len(m.completions) == 0 {
		return ""
	}
	maxShow := min(5, len(m.completions))
	var parts []string
	for i := 0; i < maxShow; i++ {
		c := m.completions[i]
		label := c.Text
		if c.Hint != "" {
			label += " " + styleCompleteHint.Render("· "+c.Hint)
		}
		if i == m.completeIdx%len(m.completions) {
			parts = append(parts, styleCompleteActive.Render(label))
		} else {
			parts = append(parts, styleCompleteBar.Render(label))
		}
	}
	if len(m.completions) > maxShow {
		parts = append(parts, styleCompleteBar.Render("…"))
	}
	return strings.Join(parts, "  ")
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing...\n"
	}
	if cached, ok := m.tryCachedView(); ok {
		return cached
	}
	start := time.Now()
	defer func() {
		if d := time.Since(start); d >= diagSlowView {
			recordViewSlow(d, 0, len(m.outputContent))
		}
	}()
	view := m.renderHeaderCached() + m.renderBody() + m.renderFooter()
	out := lipgloss.NewStyle().Width(m.width).Render(view)
	m.storeCachedView(out)
	return out
}

func (m *Model) renderStatusBar() string {
	mode := m.cfg.IssueAutomation.ModeLabel()
	model := m.cfg.AI.Model
	watch := m.todoWatchSummary()

	wh := "wh:off"
	if m.cfg.Webhook.Enabled {
		wh = "wh:on"
	}

	cwd := m.cachedCWD()

	line := fmt.Sprintf("%s · %s · %s · %s · 待办 %d", model, mode, wh, watch, m.store.ActiveCount())
	if s := strings.TrimSpace(m.invStatus); s != "" {
		if len(s) > 48 {
			s = s[:48] + "…"
		}
		line += " · " + s
	}
	if m.width > 0 {
		pad := m.width - lipgloss.Width(line) - lipgloss.Width(cwd) - 2
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
	}
	line += cwd
	return styleStatusBar.Width(m.width).Render(line)
}

func (m *Model) refreshCompletions() {
	m.completions = computeCompletions(m.input.Value(), m.activeTodos())
}

func (m *Model) resetCompletions() {
	m.completeIdx = 0
	m.completions = nil
}

func (m *Model) applyCompletionTab() bool {
	m.refreshCompletions()
	if len(m.completions) == 0 {
		return false
	}
	idx := m.completeIdx % len(m.completions)
	m.input.SetValue(m.completions[idx].Text)
	m.completeIdx++
	m.markDirty()
	return true
}

func (m *Model) applyCompletionGhost() bool {
	m.refreshCompletions()
	suffix := ghostSuffix(m.input.Value(), m.completions)
	if suffix == "" {
		return false
	}
	m.input.SetValue(m.input.Value() + suffix)
	m.resetCompletions()
	m.refreshCompletions()
	m.markDirty()
	return true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

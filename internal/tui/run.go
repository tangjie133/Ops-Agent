package tui

// run.go — TUI 入口：加载 store、启动 Webhook 运行时与 bubbletea Program。

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// Run 启动 TUI 主循环：加载 store、初始化 Webhook 运行时、进入 bubbletea Program。
func Run(cfg *config.Config) error {
	tasks, err := task.Load(config.TaskStorePath(cfg))
	if err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	startLogPersistWorker()
	diagStartupNote()
	initBackgroundLog()

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	m := NewModel(cfg, tasks, nil)
	m.runCtx = runCtx

	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithFPS(15), tea.WithMouseCellMotion())

	// Webhook 只写 store/日志，不 Post 到 TUI；UI 由 refresh tick 轮询磁盘。
	runtime := NewWebhookRuntime(cfg, tasks, nil)
	m.whRuntime = runtime

	shutdown := func() {
		runCancel()
		runtime.Shutdown()
	}

	installSignalHandler(p, shutdown)

	if _, err := p.Run(); err != nil {
		shutdown()
		return fmt.Errorf("tui: %w", err)
	}
	shutdown()
	return nil
}

package tui

// run.go — TUI 入口：加载 store、启动 Webhook 运行时与 bubbletea Program。

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// Run 启动 TUI 主循环：加载 store、初始化 Webhook 运行时、进入 bubbletea Program。
func Run(cfg *config.Config) error {
	store, err := todo.Load(config.TodoStorePath())
	if err != nil {
		return fmt.Errorf("todo store: %w", err)
	}
	libTestStore, err := libtest.Load(config.LibTestStorePath())
	if err != nil {
		return fmt.Errorf("libtest store: %w", err)
	}

	startLogPersistWorker()
	diagStartupNote()
	initBackgroundLog()

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	m := NewModel(cfg, store, libTestStore, nil)
	m.runCtx = runCtx
	m.bindInvestigatorLog()

	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithFPS(15), tea.WithMouseCellMotion())

	// Webhook 只写 store/日志，不 Post 到 TUI；UI 由 refresh tick 轮询磁盘。
	runtime := NewWebhookRuntime(cfg, store, libTestStore, nil)
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

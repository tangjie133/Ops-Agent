package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/webhook"
)

// WebhookEventMsg 由 webhook 服务投递，表示收到 GitHub 事件。
type WebhookEventMsg struct {
	Event webhook.Event
}

func Run(cfg *config.Config) error {
	store, err := todo.Load(config.TodoStorePath())
	if err != nil {
		return fmt.Errorf("todo store: %w", err)
	}

	m := NewModel(cfg, store, nil)
	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	runtime := NewWebhookRuntime(cfg, store, func(evt webhook.Event) {
		p.Send(WebhookEventMsg{Event: evt})
	})
	m.whRuntime = runtime

	if err := runtime.Start(); err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	defer runtime.Shutdown()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

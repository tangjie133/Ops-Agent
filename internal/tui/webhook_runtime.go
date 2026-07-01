package tui

// webhook_runtime.go — webhook.Runtime 的 TUI 侧别名，注入 UI logger。

import (
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/webhook"
)

type WebhookRuntime = webhook.Runtime

// NewWebhookRuntime 创建 TUI 侧 Webhook 运行时（日志写入 tui.log）。
func NewWebhookRuntime(cfg *config.Config, store *todo.FileStore, libTest *libtest.FileStore, onEvt webhook.OnEvent) *WebhookRuntime {
	return webhook.NewRuntime(cfg, store, libTest, onEvt, NewUILogger())
}

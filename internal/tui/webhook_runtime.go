package tui

import (
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/webhook"
)

type WebhookRuntime = webhook.Runtime

func NewWebhookRuntime(cfg *config.Config, store *todo.FileStore, libTest *libtest.FileStore, onEvt webhook.OnEvent) *WebhookRuntime {
	return webhook.NewRuntime(cfg, store, libTest, onEvt, NewUILogger())
}

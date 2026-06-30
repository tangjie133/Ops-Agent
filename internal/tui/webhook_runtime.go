package tui

import (
	"io"
	"log"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/webhook"
)

type WebhookRuntime = webhook.Runtime

func NewWebhookRuntime(cfg *config.Config, store *todo.FileStore, onEvt webhook.OnEvent) *WebhookRuntime {
	return webhook.NewRuntime(cfg, store, onEvt, log.New(io.Discard, "", 0))
}

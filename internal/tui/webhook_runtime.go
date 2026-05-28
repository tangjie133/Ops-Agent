package tui

import (
	"context"
	"sync"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/webhook"
)

type WebhookRuntime struct {
	mu    sync.Mutex
	cfg   *config.Config
	store *todo.FileStore
	onAdd webhook.OnEnqueue
	srv   *webhook.Server
}

func NewWebhookRuntime(cfg *config.Config, store *todo.FileStore, onAdd webhook.OnEnqueue) *WebhookRuntime {
	return &WebhookRuntime{cfg: cfg, store: store, onAdd: onAdd}
}

func (r *WebhookRuntime) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.srv = webhook.NewServer(r.cfg, r.store, r.onAdd)
	return r.srv.Start()
}

func (r *WebhookRuntime) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = r.srv.Shutdown(ctx)
	r.srv = nil
}

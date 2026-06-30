package webhook

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/smee"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// Runtime 管理本地 webhook 服务与可选的 smee 隧道。
type Runtime struct {
	mu       sync.Mutex
	cfg      *config.Config
	store    *todo.FileStore
	libTest  *libtest.FileStore
	onEvt    OnEvent
	logger   *log.Logger
	srv      *Server
	smee     *smee.Client
}

func NewRuntime(cfg *config.Config, store *todo.FileStore, libTest *libtest.FileStore, onEvt OnEvent, logger *log.Logger) *Runtime {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Runtime{cfg: cfg, store: store, libTest: libTest, onEvt: onEvt, logger: logger}
}

func (r *Runtime) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startLocked()
}

func (r *Runtime) startLocked() error {
	r.stopSmeeLocked()
	if r.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = r.srv.Shutdown(ctx)
		cancel()
		r.srv = nil
	}

	if !r.cfg.Webhook.Enabled {
		return nil
	}

	r.srv = NewServer(r.cfg, r.store, r.libTest, r.onEvt, r.logger)
	if err := r.srv.Start(); err != nil {
		r.srv = nil
		return err
	}
	r.startSmeeLocked()
	return nil
}

func (r *Runtime) Restart(cfg *config.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg = cfg
	return r.startLocked()
}

func (r *Runtime) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopSmeeLocked()
	if r.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = r.srv.Shutdown(ctx)
	r.srv = nil
}

func (r *Runtime) ListenURL() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv == nil {
		return r.cfg.Webhook.LocalURL()
	}
	return r.srv.Addr()
}

func (r *Runtime) HealthURL() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv == nil {
		return r.cfg.Webhook.HealthURL()
	}
	return r.srv.HealthURL()
}

func (r *Runtime) PayloadURL() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg.Webhook.PayloadURL()
}

func (r *Runtime) SmeeActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg.Webhook.SmeeTunnelActive()
}

func (r *Runtime) SmeeSummary() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	w := r.cfg.Webhook
	if !w.Tunnel.Smee.Enabled {
		return "已禁用"
	}
	if w.SmeeChannelURL() == "" {
		return "未配置 Public URL"
	}
	if !config.IsSmeeChannelURL(w.SmeeChannelURL()) {
		return "Public URL 非 smee.io（请改用 smee 频道或关闭隧道）"
	}
	if r.smee != nil {
		return fmt.Sprintf("已连接 → %s", w.LocalURL())
	}
	return "连接中…"
}

func (r *Runtime) startSmeeLocked() {
	if !r.cfg.Webhook.SmeeTunnelActive() {
		return
	}
	channel := r.cfg.Webhook.SmeeChannelURL()
	target := r.cfg.Webhook.LocalURL()
	r.smee = smee.NewClient(channel, target, r.logger)
	r.smee.Start(context.Background())
	r.logger.Printf("smee · 转发 %s → %s", channel, target)
}

func (r *Runtime) stopSmeeLocked() {
	if r.smee == nil {
		return
	}
	r.smee.Stop()
	r.smee = nil
}

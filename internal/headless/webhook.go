package headless

// webhook.go — OPS_AGENT_WEBHOOK_ONLY=1 时仅启动 Webhook 服务（无 TUI）。

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
	"github.com/ZzedJay/Ops-Agent/internal/webhook"
)

// ShouldRunWebhookOnly 判断是否以纯 Webhook 守护进程模式运行。
func ShouldRunWebhookOnly() bool {
	return os.Getenv("OPS_AGENT_WEBHOOK_ONLY") == "1"
}

func RunWebhookOnly(cfg *config.Config) int {
	if !cfg.Webhook.Enabled {
		fmt.Fprintln(os.Stderr, "webhook: disabled in config")
		return 1
	}

	store, err := todo.Load(config.TodoStorePath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "todo store: %v\n", err)
		return 1
	}
	libTestStore, err := libtest.Load(config.LibTestStorePath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "libtest store: %v\n", err)
		return 1
	}

	srv := webhook.NewRuntime(cfg, store, libTestStore, func(evt webhook.Event) {
		log.Printf("%s", evt.Message())
	}, log.Default())
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "webhook: %v\n", err)
		return 1
	}
	defer srv.Shutdown()

	log.Printf("webhook-only mode: %s", srv.ListenURL())
	log.Printf("health: %s", srv.HealthURL())
	if payload := srv.PayloadURL(); payload != "" {
		log.Printf("github payload url: %s", payload)
	}
	if cfg.Webhook.SmeeTunnelActive() {
		log.Printf("smee tunnel: %s", srv.SmeeSummary())
	}
	log.Printf("todo store: %s", config.TodoStorePath())
	log.Printf("press Ctrl+C to stop")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	return 0
}

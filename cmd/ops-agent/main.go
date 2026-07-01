// Ops-Agent 主程序入口。
//
// 运行模式（按优先级）：
//  1. OPS_AGENT_WEBHOOK_ONLY=1 — 仅启动 Webhook 服务（无 TUI）
//  2. 非 TTY / CI 环境 — headless 模式（PR 检测等）
//  3. 默认 — 交互式 TUI
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/headless"
	"github.com/ZzedJay/Ops-Agent/internal/tui"
)

func main() {
	// 加载 ~/.config/ops-agent/config.yaml（或环境变量指定路径）
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if headless.ShouldRunWebhookOnly() {
		code := headless.RunWebhookOnly(cfg)
		if code != 0 {
			fmt.Fprintln(os.Stderr, "ops-agent: webhook-only run failed")
		}
		os.Exit(code)
	}

	if headless.ShouldRun() {
		code := headless.Run(cfg)
		if code != 0 {
			fmt.Fprintln(os.Stderr, "ops-agent: headless run failed")
		}
		os.Exit(code)
	}

	if err := tui.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ops-agent: %v\n", err)
		os.Exit(1)
	}
}

// Ops-Agent 主程序入口。v1 只启动 TUI；TUI 关闭后进程结束。
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := tui.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ops-agent: %v\n", err)
		os.Exit(1)
	}
}

package headless

// run.go — 非 TTY / CI 环境下的 headless 入口（PR 检测等）。

import (
	"os"

	"golang.org/x/term"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// ShouldRun 判断是否应跳过 TUI（CI 或非终端 stdout）。
func ShouldRun() bool {
	if os.Getenv("OPS_AGENT_CI") == "1" {
		return true
	}
	if os.Getenv("CI") == "true" {
		return true
	}
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

// Run 执行 headless 主流程并返回退出码。
func Run(cfg *config.Config) int {
	return runPRCheck(cfg)
}

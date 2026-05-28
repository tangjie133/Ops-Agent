package headless

import (
	"os"

	"golang.org/x/term"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func ShouldRun() bool {
	if os.Getenv("OPS_AGENT_CI") == "1" {
		return true
	}
	if os.Getenv("CI") == "true" {
		return true
	}
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

func Run(cfg *config.Config) int {
	return runPRCheck(cfg)
}

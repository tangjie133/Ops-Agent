package headless

import (
	"fmt"
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
	if os.Getenv("OPS_AGENT_ISSUE_SCAN") == "1" {
		return runIssueScan(cfg)
	}
	return runPRCheck(cfg)
}

func runPRCheck(cfg *config.Config) int {
	fmt.Println("ops-agent headless: PR check (M2 — not implemented yet)")
	fmt.Printf("  GITHUB_REPOSITORY=%s\n", os.Getenv("GITHUB_REPOSITORY"))
	fmt.Printf("  notify on_failure: %v\n", cfg.Notify.OnFailure)
	return 0
}

func runIssueScan(cfg *config.Config) int {
	fmt.Println("ops-agent headless: issue scan (M4 — not implemented yet)")
	fmt.Printf("  labels: %v\n", cfg.IssueWatch.Labels)
	return 0
}

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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
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

package headless

import (
	"context"
	"fmt"
	"os"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/notify"
	"github.com/ZzedJay/Ops-Agent/internal/prcheck"
)

// runPRCheck 是 M2 唯一 headless 默认路径：检测 →（失败时）notify → exit code。
func runPRCheck(cfg *config.Config) int {
	ctx := context.Background()
	gh := github.NewClient()

	opts := prcheck.Options{
		Repo:     repoFromEnv(),
		PRNumber: prNumberFromEnv(),
	}
	res, err := prcheck.Check(ctx, gh, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ops-agent: pr check error: %v\n", err)
		return 1
	}

	fmt.Print(res.FormatReport())
	if res.OK {
		return 0
	}

	if cfg.Notify.OnFailure {
		n := notify.FromAppConfig(cfg)
		alert := res.ToAlert(runURLFromEnv())
		if err := n.Send(ctx, alert); err != nil {
			fmt.Fprintf(os.Stderr, "ops-agent: notify failed: %v\n", err)
		}
	}
	return 1
}

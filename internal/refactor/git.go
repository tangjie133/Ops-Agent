package refactor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

func gitRun(ctx context.Context, proxy config.ProxyConfig, dir string, args ...string) ([]byte, error) {
	gitArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	netproxy.ConfigureCmd(cmd, proxy)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func checkoutBranch(ctx context.Context, proxy config.ProxyConfig, dir, branch string, create bool) error {
	args := []string{"checkout"}
	if create {
		args = append(args, "-B")
	}
	args = append(args, branch)
	if _, err := gitRun(ctx, proxy, dir, args...); err != nil {
		return err
	}
	return nil
}

func hasWorkingTreeChanges(ctx context.Context, proxy config.ProxyConfig, dir string) (bool, error) {
	out, err := gitRun(ctx, proxy, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func commitAll(ctx context.Context, proxy config.ProxyConfig, dir, message string) error {
	if _, err := gitRun(ctx, proxy, dir, "add", "-A"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, proxy, dir, "commit", "-m", message); err != nil {
		return err
	}
	return nil
}

func pushBranch(ctx context.Context, proxy config.ProxyConfig, dir, branch string) error {
	if _, err := gitRun(ctx, proxy, dir, "push", "-u", "origin", branch); err != nil {
		return err
	}
	return nil
}

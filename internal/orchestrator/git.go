package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

func porcelain(workDir string) (deleted int, paths []string, dirty bool, err error) {
	cmd := exec.Command("git", "-C", workDir, "status", "--porcelain", "-uall")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, nil, false, fmt.Errorf("git status: %s", strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		dirty = true
		code := ""
		if len(line) >= 2 {
			code = line[:2]
		}
		path := strings.TrimSpace(line[2:])
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		paths = append(paths, path)
		if strings.Contains(code, "D") {
			deleted++
		}
	}
	return deleted, paths, dirty, nil
}

func commitAndPush(ctx context.Context, workDir, branch string, proxy config.ProxyConfig) error {
	cmds := [][]string{
		{"git", "-C", workDir, "checkout", "-B", branch},
		{"git", "-C", workDir, "add", "-A"},
		{"git", "-C", workDir, "-c", "user.email=ops-agent@local", "-c", "user.name=Ops-Agent", "commit", "-m", branch},
	}
	for _, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
		}
	}
	push := exec.CommandContext(ctx, "git", "-C", workDir, "push", "-u", "origin", "HEAD")
	netproxy.ConfigureCmd(push, proxy)
	if out, err := push.CombinedOutput(); err != nil {
		return fmt.Errorf("git push: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

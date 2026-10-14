package verifier

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Opts 是一次验证的输入。
type Opts struct {
	RepoPath string
	Commands []string
	Runner   func(ctx context.Context, dir, command string) (string, error)
}

// Result 是验证报告。
type Result struct {
	OK      bool
	Report  string
	Command string
}

// Run 执行命令列表。没有命令时视为缺少验证。
func Run(ctx context.Context, opts Opts) Result {
	if len(opts.Commands) == 0 {
		return Result{OK: false, Report: "缺少验证"}
	}
	res := Result{OK: true}
	var parts []string
	run := opts.Runner
	if run == nil {
		run = defaultRunner
	}
	for _, command := range opts.Commands {
		out, err := run(ctx, opts.RepoPath, command)
		parts = append(parts, command+"\n"+out)
		if err != nil {
			res.OK = false
			res.Command = command
			parts = append(parts, err.Error())
			break
		}
	}
	res.Report = strings.TrimSpace(strings.Join(parts, "\n"))
	return res
}

func defaultRunner(ctx context.Context, dir, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("command failed: %w", err)
	}
	return text, nil
}

package verifier

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/repovalidate"
)

// Opts 是一次验证的输入。
type Opts struct {
	RepoPath     string
	StandardsDir string
	Standard     string
	Commands     []string
	Runner       func(ctx context.Context, dir, command string) (string, error)
}

// Result 是验证报告。
type Result struct {
	OK      bool
	Report  string
	Command string
}

// Run 执行仓库规范检查和命令列表。
func Run(ctx context.Context, opts Opts) Result {
	res := Result{OK: true}
	var parts []string
	if opts.Standard != "" {
		std, err := repovalidate.LoadStandard(opts.StandardsDir, opts.Standard)
		if err != nil {
			res.OK = false
			res.Report = err.Error()
			return res
		}
		report := repovalidate.Validate(opts.RepoPath, std)
		parts = append(parts, report.Format())
		if !report.OK {
			res.OK = false
		}
	}
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

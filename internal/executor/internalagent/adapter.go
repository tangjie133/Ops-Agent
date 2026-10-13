package internalagent

import (
	"context"
	"os/exec"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/executor"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// Engine 是可替换的重构引擎。
type Engine interface {
	Run(ctx context.Context, repoPath string, item todo.Item, issue *github.Issue) (string, error)
}

// Adapter 把统一执行请求转成现有重构引擎调用。
type Adapter struct {
	Engine Engine
}

// Run 调用底层引擎，并根据摘要或工作区脏状态判断是否有改动。
func (a Adapter) Run(ctx context.Context, req executor.Request) (executor.Result, error) {
	summary, err := a.Engine.Run(ctx, req.WorkDir, todo.Item{
		Repo: req.Repo, Number: req.Number, Title: req.Title,
	}, &github.Issue{Number: req.Number, Title: req.Title, Body: req.Body})
	if err != nil {
		return executor.Result{}, err
	}
	changed := strings.TrimSpace(summary) != "" || dirty(req.WorkDir)
	return executor.Result{Summary: summary, Changed: changed}, nil
}

func dirty(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

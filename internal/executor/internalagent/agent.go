package internalagent

// agent.go — 在编排器给出的工作区里只读调查，产出 Issue 评论正文。

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/executor"
)

// LLM 是可替换的对话接口，测试注入假实现。
type LLM interface {
	ChatMessages(ctx context.Context, msgs []ai.Message) (string, error)
}

// Agent 根据 Issue 阅读仓库并生成评论。
type Agent struct {
	llm LLM
}

// New 使用配置里的模型。
func New(cfg *config.Config) *Agent {
	var llm LLM
	if cfg != nil {
		llm = ai.NewClient(cfg.AI)
	}
	return &Agent{llm: llm}
}

// Run 实现 executor.Executor。
func (a Agent) Run(ctx context.Context, req executor.Request) (executor.Result, error) {
	if a.llm == nil {
		return executor.Result{}, fmt.Errorf("llm not configured")
	}
	if strings.TrimSpace(req.WorkDir) == "" {
		return executor.Result{}, fmt.Errorf("work dir required")
	}
	summary, err := runLoop(ctx, a.llm, req)
	if err != nil {
		return executor.Result{}, err
	}
	return executor.Result{Summary: summary, Changed: gitDirty(req.WorkDir)}, nil
}

func gitDirty(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func issuePrompt(req executor.Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "仓库: %s\nIssue: #%d\n标题: %s\n\n正文:\n%s\n",
		req.Repo, req.Number, req.Title, strings.TrimSpace(req.Body))
	b.WriteString("\n工作目录是该仓库副本。问答不要改文件；缺陷用 edit_file 后 reply。不要自己跑测试。")
	return b.String()
}

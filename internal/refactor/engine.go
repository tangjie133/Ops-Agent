package refactor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/investigator"
	"github.com/ZzedJay/Ops-Agent/internal/rag"
	"github.com/ZzedJay/Ops-Agent/internal/repocontext"
	"github.com/ZzedJay/Ops-Agent/internal/repovalidate"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type llmAdapter struct {
	client *ai.Client
}

func (a llmAdapter) ChatMessages(ctx context.Context, msgs []investigator.Message) (string, error) {
	out := make([]ai.Message, len(msgs))
	for i, m := range msgs {
		out[i] = ai.Message{Role: m.Role, Content: m.Content}
	}
	return a.client.ChatMessages(ctx, out)
}

// Engine 在功能分支上运行 AI 重构循环。
type Engine struct {
	cfg        config.AIConfig
	refactorPR config.RefactorPRConfig
	proxy      config.ProxyConfig
	gh         *github.Client
	workspace  *repocontext.Workspace
	log        investigator.Logger
}

func NewEngine(cfg *config.Config, gh *github.Client) *Engine {
	cfg.IssueAutomation.RefactorPR.Normalize()
	cfg.Proxy.Normalize()
	return &Engine{
		cfg:        cfg.AI,
		refactorPR: cfg.IssueAutomation.RefactorPR,
		proxy:      cfg.Proxy,
		gh:         gh,
		workspace:  repocontext.NewWorkspace(cfg.AI.RepoContext, cfg.Proxy, gh),
	}
}

func (e *Engine) SetLogger(log investigator.Logger) {
	e.log = log
}

func (e *Engine) Run(ctx context.Context, repoPath string, item todo.Item, issue *github.Issue) (summary string, err error) {
	if e.gh == nil {
		return "", fmt.Errorf("github client not configured")
	}

	var ragIdx *rag.Index
	if e.cfg.RAG.On() && e.cfg.RAG.ReindexOnAnalyzeOn() {
		idx, idxErr := rag.EnsureKnowledgeIndex(ctx, e.cfg.RAG, nil)
		if idxErr == nil {
			ragIdx = idx
		}
	}

	tools := NewToolbox(repoPath, e.cfg.Investigator, e.cfg.RAG, ragIdx, e.proxy)
	tools.SetLogger(e.log)

	invCfg := e.cfg.Investigator
	if e.refactorPR.MaxSteps > 0 {
		invCfg.MaxSteps = e.refactorPR.MaxSteps
	}
	if invCfg.MaxSteps <= 0 {
		invCfg.MaxSteps = 12
	}
	loop := NewLoop(invCfg, llmAdapter{client: ai.NewClient(e.cfg)}, tools)
	loop.SetLogger(e.log)

	prompt := buildRefactorPrompt(item, issue)
	return loop.Run(ctx, prompt)
}

func buildRefactorPrompt(item todo.Item, issue *github.Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "仓库: %s\nIssue: #%d\n标题: %s\n链接: %s\n\nIssue 正文:\n%s",
		item.Repo, item.Number, item.Title, item.URL, strings.TrimSpace(issue.Body))
	if draft := strings.TrimSpace(item.Draft); draft != "" {
		b.WriteString("\n\n── 已有分析草稿（参考修复方向）──\n")
		b.WriteString(draft)
	}
	b.WriteString("\n\n请在当前分支上实现修复，运行测试与 repo_validate，完成后 done。")
	b.WriteString("\n\n修改代码时：只改与 Issue 相关的部分；对已有文件使用 edit_file 的 old/new 局部替换，勿整文件覆盖或删除无关函数。")
	return b.String()
}

func defaultTestCommands(repoPath string) []string {
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		return []string{"go test ./..."}
	}
	return nil
}

func runTestCommands(ctx context.Context, proxy config.ProxyConfig, repoPath string, commands []string) error {
	if len(commands) == 0 {
		return nil
	}
	tools := NewToolbox(repoPath, config.InvestigatorConfig{}, config.RAGConfig{}, nil, proxy)
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		if _, err := tools.runCmd(ctx, cmd); err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
	}
	return nil
}

func validateRepo(repoPath string, ragCfg config.RAGConfig) error {
	if ragCfg.DefaultStandard == "" {
		return nil
	}
	standardsDir := filepath.Join(config.KnowledgeDir(ragCfg), "standards")
	std, err := repovalidate.LoadStandard(standardsDir, ragCfg.DefaultStandard)
	if err != nil {
		return err
	}
	report := repovalidate.Validate(repoPath, std)
	if !report.OK {
		return fmt.Errorf("repo_validate failed:\n%s", report.Format())
	}
	return nil
}

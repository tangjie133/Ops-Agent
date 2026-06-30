package investigator

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/repocontext"
)

// Investigator 多轮 Agent Issue 分析器。
type Investigator struct {
	cfg       config.AIConfig
	gh        *github.Client
	llm       LLM
	workspace *repocontext.Workspace
	observer  StepObserver
}

func New(cfg config.AIConfig, gh *github.Client, llm LLM) *Investigator {
	cfg.Investigator.Normalize()
	cfg.RepoContext.Normalize()
	return &Investigator{
		cfg:       cfg,
		gh:        gh,
		llm:       llm,
		workspace: repocontext.NewWorkspace(cfg.RepoContext, gh),
	}
}

func (inv *Investigator) SetObserver(obs StepObserver) {
	inv.observer = obs
}

// AnalyzeIssue 调查仓库并生成 GitHub 评论草稿。
func (inv *Investigator) AnalyzeIssue(ctx context.Context, repo string, num int) (string, error) {
	if inv.gh == nil {
		return "", fmt.Errorf("github client not configured")
	}
	if inv.llm == nil {
		return "", fmt.Errorf("llm not configured")
	}

	iss, err := inv.gh.IssueView(ctx, repo, num)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(iss.State, "OPEN") {
		return "", fmt.Errorf("issue %s#%d is not open", repo, num)
	}

	repoPath, err := inv.workspace.Prepare(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("prepare repository: %w", err)
	}

	tools := NewToolbox(repoPath, inv.cfg.Investigator)
	loop := NewLoop(inv.cfg.Investigator, inv.llm, tools, inv.observer)

	return loop.Run(ctx, buildIssuePrompt(repo, iss))
}

func buildIssuePrompt(repo string, iss *github.Issue) string {
	var labels []string
	for _, l := range iss.Labels {
		labels = append(labels, l.Name)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "仓库: %s\nIssue: #%d\n标题: %s\n状态: %s\n标签: %s\n链接: %s\n\n正文:\n%s",
		repo, iss.Number, iss.Title, iss.State, strings.Join(labels, ", "), iss.URL, strings.TrimSpace(iss.Body))

	if len(iss.Comments) > 0 {
		b.WriteString("\n\n── Issue 评论 ──\n")
		for i, c := range iss.Comments {
			author := c.Author.Login
			if author == "" {
				author = "unknown"
			}
			fmt.Fprintf(&b, "\n[评论 %d — @%s]\n%s\n", i+1, author, strings.TrimSpace(c.Body))
		}
	}

	b.WriteString("\n\n请调查仓库源码，完成后用 reply action 输出 GitHub 评论。")
	return b.String()
}

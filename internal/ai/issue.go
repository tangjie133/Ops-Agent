package ai

import (
	"context"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/investigator"
)

// llmAdapter 将 ai.Client 适配为 investigator.LLM。
type llmAdapter struct {
	client *Client
}

func (a llmAdapter) ChatMessages(ctx context.Context, msgs []investigator.Message) (string, error) {
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[i] = Message{Role: m.Role, Content: m.Content}
	}
	return a.client.ChatMessages(ctx, out)
}

// IssueAnalyzer 通过多轮 Agent 分析 Issue 并生成回复草稿。
type IssueAnalyzer struct {
	inv *investigator.Investigator
}

func NewIssueAnalyzer(cfg config.AIConfig, gh *github.Client) *IssueAnalyzer {
	client := NewClient(cfg)
	return &IssueAnalyzer{inv: investigator.New(cfg, gh, llmAdapter{client: client})}
}

func (a *IssueAnalyzer) AnalyzeIssue(ctx context.Context, repo string, num int) (string, error) {
	return a.inv.AnalyzeIssue(ctx, repo, num)
}

// FormatCommentBody 附加自动回复 footer。
func FormatCommentBody(draft string, cfg *config.Config) string {
	body := strings.TrimSpace(draft)
	if cfg == nil {
		return body
	}
	footer := strings.TrimSpace(cfg.IssueAutomation.AutoReply.CommentFooter)
	if footer == "" {
		return body
	}
	if body == "" {
		return footer
	}
	return body + "\n\n" + footer
}

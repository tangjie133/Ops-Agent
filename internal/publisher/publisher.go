package publisher

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/github"
)

// GitHubAPI 是写回所需的最小 GitHub 能力。
type GitHubAPI interface {
	PRCreate(ctx context.Context, opts github.PRCreateOpts) (string, error)
	IssueComment(ctx context.Context, repo string, num int, body string) error
}

// DraftPRInput 是创建 draft PR 的参数。
type DraftPRInput struct {
	Repo   string
	Number int
	Title  string
	Head   string
	Base   string
	Body   string
	TaskID string
	RunID  string
}

// Publisher 只暴露允许自动执行的写回动作。
type Publisher struct {
	gh GitHubAPI
}

// New 创建写回器。
func New(gh GitHubAPI) *Publisher {
	return &Publisher{gh: gh}
}

// Footer 生成可追踪的写回页脚。
func Footer(taskID, runID string) string {
	return fmt.Sprintf("---\nHandled by Ops-Agent\nTask: %s\nRun: %s\nMode: auto", taskID, runID)
}

// CreateDraftPR 创建草稿 PR，不合并、不关闭 Issue。
func (p *Publisher) CreateDraftPR(ctx context.Context, in DraftPRInput) (string, error) {
	body := strings.TrimSpace(in.Body) + "\n\n" + Footer(in.TaskID, in.RunID)
	return p.gh.PRCreate(ctx, github.PRCreateOpts{
		Repo:  in.Repo,
		Title: in.Title,
		Body:  body,
		Base:  in.Base,
		Head:  in.Head,
		Draft: true,
	})
}

// CommentIssue 在 Issue 上留下带页脚的说明。
func (p *Publisher) CommentIssue(ctx context.Context, repo string, number int, body, taskID, runID string) error {
	return p.gh.IssueComment(ctx, repo, number, strings.TrimSpace(body)+"\n\n"+Footer(taskID, runID))
}

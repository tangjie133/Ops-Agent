package pr

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const describeSystemPrompt = `你是 Pull Request 撰写助手。根据 git 提交记录与 diff 统计，生成简洁专业的 PR 标题与正文（中文为主，技术术语可保留英文）。

输出格式必须严格为：
TITLE: 一行标题（不超过 72 字符）
BODY:
（Markdown 正文：## 变更摘要、## 测试说明；若 commit 含 #数字 可写 Fixes #n）

不要输出其它前缀或解释。`

// Draft AI 生成的 PR 草稿。
type Draft struct {
	Repo          string
	Title         string
	Body          string
	BaseBranch    string
	HeadBranch    string
	ExistingPR    int // 非 0 表示更新已有 PR
	ExistingURL   string
	CommitCount   int
	NeedsPush     bool
	UnpushedCount int
	PushHint      string
}

// GenerateDraft 调用 AI 根据 BranchInfo 生成 PR 标题与正文。
func GenerateDraft(ctx context.Context, aiCfg config.AIConfig, info *BranchInfo) (*Draft, error) {
	if info == nil {
		return nil, fmt.Errorf("nil branch info")
	}
	client := ai.NewClient(aiCfg)
	user := buildDescribePrompt(info)
	raw, err := client.Chat(ctx, describeSystemPrompt, user)
	if err != nil {
		return nil, err
	}
	title, body, err := parseDescribeResponse(raw)
	if err != nil {
		return nil, err
	}
	d := &Draft{
		Repo:          info.Repo,
		Title:         title,
		Body:          body,
		BaseBranch:    info.BaseBranch,
		HeadBranch:    info.HeadBranch,
		CommitCount:   info.CommitCount,
		NeedsPush:     info.NeedsPush,
		UnpushedCount: info.UnpushedCount,
		PushHint:      info.PushHint,
	}
	if info.ExistingPR != nil {
		d.ExistingPR = info.ExistingPR.Number
		d.ExistingURL = info.ExistingPR.URL
	}
	return d, nil
}

func buildDescribePrompt(info *BranchInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "仓库: %s\n", info.Repo)
	fmt.Fprintf(&b, "分支: %s → %s\n\n", info.HeadBranch, info.BaseBranch)
	if info.ExistingPR != nil {
		fmt.Fprintf(&b, "已有 PR #%d: %s\n\n", info.ExistingPR.Number, info.ExistingPR.Title)
	}
	if strings.TrimSpace(info.Commits) != "" {
		b.WriteString("提交记录:\n")
		b.WriteString(info.Commits)
		b.WriteString("\n\n")
	} else {
		b.WriteString("提交记录: （无相对默认分支的新提交）\n\n")
	}
	if strings.TrimSpace(info.DiffStat) != "" {
		b.WriteString("Diff 统计:\n")
		b.WriteString(info.DiffStat)
	} else {
		b.WriteString("Diff 统计: （空）\n")
	}
	return b.String()
}

func parseDescribeResponse(raw string) (title, body string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("AI 返回空内容")
	}
	const titlePrefix = "TITLE:"
	const bodyPrefix = "BODY:"
	titleIdx := strings.Index(raw, titlePrefix)
	bodyIdx := strings.Index(raw, bodyPrefix)
	if titleIdx >= 0 && bodyIdx > titleIdx {
		titlePart := strings.TrimSpace(raw[titleIdx+len(titlePrefix) : bodyIdx])
		titlePart = strings.TrimSpace(strings.SplitN(titlePart, "\n", 2)[0])
		body = strings.TrimSpace(raw[bodyIdx+len(bodyPrefix):])
		if titlePart != "" && body != "" {
			return titlePart, body, nil
		}
	}
	//  fallback: 首行标题，其余正文
	lines := strings.Split(raw, "\n")
	title = strings.TrimSpace(lines[0])
	title = strings.TrimPrefix(title, "# ")
	if title == "" {
		return "", "", fmt.Errorf("无法解析 PR 标题")
	}
	if len(lines) > 1 {
		body = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	if body == "" {
		body = title
	}
	return title, body, nil
}

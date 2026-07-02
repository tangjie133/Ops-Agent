package issuecomment

import (
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/github"
)

// IsAgentReply 判断评论是否为 Ops-Agent 自动回复（与 webhook 忽略逻辑一致）。
func IsAgentReply(body, footer string) bool {
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}
	footer = strings.TrimSpace(footer)
	if footer != "" && strings.Contains(body, footer) {
		return true
	}
	return strings.Contains(body, "_Posted by Ops-Agent")
}

// Selection 评论筛选结果统计。
type Selection struct {
	Comments      []github.IssueComment
	Total         int
	ExcludedAgent int
}

// SelectRecent 排除 Agent 回复后，保留时间顺序下的最近 maxN 条用户评论。
// maxN <= 0 表示不限制条数（仍排除 Agent）。
func SelectRecent(comments []github.IssueComment, maxN int, agentFooter string) Selection {
	sel := Selection{Total: len(comments)}
	var filtered []github.IssueComment
	for _, c := range comments {
		if IsAgentReply(c.Body, agentFooter) {
			sel.ExcludedAgent++
			continue
		}
		filtered = append(filtered, c)
	}
	if maxN > 0 && len(filtered) > maxN {
		filtered = filtered[len(filtered)-maxN:]
	}
	sel.Comments = filtered
	return sel
}

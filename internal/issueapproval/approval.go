package issueapproval

import "strings"

const ApprovePRCommand = "/approve-pr"

// IsApprovePRComment 判断 Issue 评论是否为 PR 重构授权（仅认 /approve-pr，整行匹配）。
func IsApprovePRComment(body string) bool {
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), ApprovePRCommand) {
			return true
		}
	}
	return false
}

package webhook

import (
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/issuecomment"
)

// skipOwnAutoReply 忽略 Ops-Agent 自动回复触发的 issue_comment，避免 full 模式循环发帖。
func skipOwnAutoReply(cfg *config.Config, c Comment) bool {
	if c.User.Type == "Bot" {
		return true
	}
	return issuecomment.IsAgentReply(c.Body, cfg.IssueAutomation.AutoReply.CommentFooter)
}

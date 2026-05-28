package issuewatch

import (
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
)

// Matches 判断 open issue 是否命中监视规则（label OR + 可选未指派）。
func Matches(iss github.Issue, cfg *config.Config) bool {
	if iss.State != "" && iss.State != "OPEN" {
		return false
	}
	if cfg.IssueWatch.RequireUnassigned && len(iss.Assignees) > 0 {
		return false
	}
	return labelMatch(iss.Labels, cfg.IssueWatch.Labels)
}

func labelMatch(issueLabels []github.Label, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(issueLabels))
	for _, l := range issueLabels {
		set[l.Name] = struct{}{}
	}
	for _, want := range required {
		if _, ok := set[want]; ok {
			return true
		}
	}
	return false
}

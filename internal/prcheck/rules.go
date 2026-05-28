package prcheck

import "github.com/ZzedJay/Ops-Agent/internal/github"

// MVP 检测项（M2）：checks 全绿 + 无 merge conflict。
// 后续（M3+）可扩展：reviews、落后 base、Fixes #n 等，见 README §8.1。
func evaluate(pr *github.PullRequest) []string {
	var failures []string

	if pr.Mergeable == "CONFLICTING" {
		failures = append(failures, "PR 存在合并冲突")
	}
	switch pr.MergeStateStatus {
	case "DIRTY", "UNSTABLE":
		failures = append(failures, "合并状态: "+pr.MergeStateStatus)
	}

	switch pr.StatusCheckRollup.State {
	case "FAILURE", "ERROR":
		for _, c := range pr.StatusCheckRollup.Contexts {
			if c.State == "FAILURE" || c.State == "ERROR" {
				failures = append(failures, "check 失败: "+c.Context+" ("+c.State+")")
			}
		}
		if len(failures) == 0 {
			failures = append(failures, "checks 状态: "+pr.StatusCheckRollup.State)
		}
	case "PENDING":
		failures = append(failures, "checks 仍在进行中 (PENDING)")
	case "SUCCESS", "":
		// ok
	default:
		failures = append(failures, "checks 状态: "+pr.StatusCheckRollup.State)
	}

	return failures
}

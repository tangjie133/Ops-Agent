package safety

import "strings"

// Verdict 是安全评估结论。
type Verdict string

const (
	VerdictNeedsMoreFix   Verdict = "needs_more_fix"
	VerdictPublishAsDraft Verdict = "publish_as_draft"
	VerdictBlock          Verdict = "block"
)

// Input 是 v1 风险评估所需的事实。
type Input struct {
	Deleted  int
	Paths    []string
	VerifyOK bool
	Retries  int
}

// Report 是风险评估输出。
type Report struct {
	Verdict Verdict
	Reason  string
}

// Evaluate 按路线图第 3 节判断能否开 draft PR。
func Evaluate(in Input) Report {
	if in.Deleted > 0 {
		return Report{Verdict: VerdictBlock, Reason: "diff 删除了文件"}
	}
	for _, path := range in.Paths {
		if highRiskPath(path) {
			return Report{Verdict: VerdictBlock, Reason: "改动了高风险路径 " + path}
		}
	}
	if !in.VerifyOK {
		if in.Retries >= 3 {
			return Report{Verdict: VerdictBlock, Reason: "连续 3 次测试失败"}
		}
		return Report{Verdict: VerdictNeedsMoreFix, Reason: "测试失败"}
	}
	return Report{Verdict: VerdictPublishAsDraft, Reason: "测试通过"}
}

func highRiskPath(path string) bool {
	p := strings.ToLower(strings.TrimSpace(path))
	needles := []string{
		".github/workflows/",
		".goreleaser",
		"codeowners",
		"secret",
		".env",
		"credential",
		"/release",
	}
	for _, n := range needles {
		if strings.Contains(p, n) {
			return true
		}
	}
	return false
}

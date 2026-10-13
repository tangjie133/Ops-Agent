package safety

// Verdict 是安全评估结论。
type Verdict string

const (
	VerdictSafeToPublish     Verdict = "safe_to_publish"
	VerdictPublishAsDraft    Verdict = "publish_as_draft"
	VerdictNeedsMoreFix      Verdict = "needs_more_fix"
	VerdictNeedsManualReview Verdict = "needs_manual_review"
	VerdictRecommendRollback Verdict = "recommend_rollback"
	VerdictBlock             Verdict = "block"
)

// Input 是一次风险评估所需的事实。
type Input struct {
	DiffFiles  int
	Deleted    int
	VerifyOK   bool
	Retries    int
	MaxRetries int
}

// Report 是风险评估输出。
type Report struct {
	Verdict Verdict
}

// Evaluate 按固定优先级给出结论。第一轮通过的修改也只建议 draft PR。
func Evaluate(in Input) Report {
	if in.Deleted >= 10 {
		return Report{Verdict: VerdictBlock}
	}
	if !in.VerifyOK && in.Retries >= in.MaxRetries && in.DiffFiles >= 15 {
		return Report{Verdict: VerdictRecommendRollback}
	}
	if !in.VerifyOK && in.Retries < in.MaxRetries {
		return Report{Verdict: VerdictNeedsMoreFix}
	}
	if !in.VerifyOK {
		return Report{Verdict: VerdictNeedsManualReview}
	}
	return Report{Verdict: VerdictPublishAsDraft}
}

package safety

import "testing"

func TestEvaluateRules(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want Verdict
	}{
		{"delete block", Input{Deleted: 10, VerifyOK: true}, VerdictBlock},
		{"rollback after retries", Input{VerifyOK: false, Retries: 3, MaxRetries: 3, DiffFiles: 15}, VerdictRecommendRollback},
		{"retry while budget remains", Input{VerifyOK: false, Retries: 1, MaxRetries: 3, DiffFiles: 2}, VerdictNeedsMoreFix},
		{"manual when retries exhausted but diff small", Input{VerifyOK: false, Retries: 3, MaxRetries: 3, DiffFiles: 2}, VerdictNeedsManualReview},
		{"large passing diff is draft", Input{VerifyOK: true, DiffFiles: 20}, VerdictPublishAsDraft},
		{"small passing diff is still draft", Input{VerifyOK: true, DiffFiles: 1}, VerdictPublishAsDraft},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(tc.in)
			if got.Verdict != tc.want {
				t.Fatalf("got %s want %s", got.Verdict, tc.want)
			}
		})
	}
}

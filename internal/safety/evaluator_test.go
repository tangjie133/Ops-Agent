package safety

import "testing"

func TestEvaluateV1(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want Verdict
	}{
		{"any delete blocks", Input{Deleted: 1, VerifyOK: true}, VerdictBlock},
		{"workflow path blocks", Input{VerifyOK: true, Paths: []string{".github/workflows/ci.yml"}}, VerdictBlock},
		{"release path blocks", Input{VerifyOK: true, Paths: []string{".goreleaser.yaml"}}, VerdictBlock},
		{"secret path blocks", Input{VerifyOK: true, Paths: []string{"config/secrets.env"}}, VerdictBlock},
		{"third failure stops", Input{VerifyOK: false, Retries: 3}, VerdictBlock},
		{"earlier failure retries", Input{VerifyOK: false, Retries: 2}, VerdictNeedsMoreFix},
		{"clean pass publishes draft", Input{VerifyOK: true, Paths: []string{"internal/foo.go"}}, VerdictPublishAsDraft},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Evaluate(tc.in).Verdict; got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

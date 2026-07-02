package issueapproval

import "testing"

func TestIsApprovePRComment(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"/approve-pr", true},
		{"  /approve-pr  ", true},
		{"请\n/approve-pr", true},
		{"请 /approve-pr\n谢谢", false},
		{"同意", false},
		{"/approve-pr please", false},
		{"LGTM", false},
	}
	for _, tc := range cases {
		if got := IsApprovePRComment(tc.body); got != tc.want {
			t.Errorf("IsApprovePRComment(%q) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

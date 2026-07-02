package tui

import "testing"

func TestIsPRDescribeIntent(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"/describe", true},
		{"/pr", true},
		{"帮我创建 PR", true},
		{"写 pr 描述", true},
		{"create pull request", true},
		{"分析一下", false},
		{"/help", false},
	}
	for _, tc := range cases {
		if got := isPRDescribeIntent(tc.line); got != tc.want {
			t.Errorf("isPRDescribeIntent(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

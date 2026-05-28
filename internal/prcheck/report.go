package prcheck

import (
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/notify"
)

func (r *Result) FormatReport() string {
	var b strings.Builder
	b.WriteString("── PR 检测 ──\n\n")
	fmt.Fprintf(&b, "仓库: %s\n", r.Repo)
	fmt.Fprintf(&b, "PR: #%d %s\n", r.PRNumber, r.PRTitle)
	if r.PRURL != "" {
		fmt.Fprintf(&b, "链接: %s\n\n", r.PRURL)
	}
	if r.OK {
		b.WriteString("结果: 通过 ✓\n")
		return b.String()
	}
	b.WriteString("结果: 失败 ✗\n\n")
	for _, f := range r.Failures {
		b.WriteString("  - " + f + "\n")
	}
	b.WriteString("\n（CI headless 失败时将自动推送 notify；TUI 内仅展示报告）\n")
	return b.String()
}

func (r *Result) ToAlert(runURL string) notify.Alert {
	return notify.Alert{
		Title:    fmt.Sprintf("[FAILED] PR #%d checks", r.PRNumber),
		Repo:     r.Repo,
		PRNumber: r.PRNumber,
		PRURL:    r.PRURL,
		Failures: r.Failures,
		RunURL:   runURL,
	}
}

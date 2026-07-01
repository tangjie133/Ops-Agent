package notify

// alert.go — 失败/就绪等告警的结构化消息与 Notifier 接口。

import (
	"context"
	"fmt"
	"strings"
)

type Alert struct {
	Title    string
	Repo     string
	PRNumber int
	PRURL    string
	Failures []string
	RunURL   string
	Issues   []IssueSummary
}

type IssueSummary struct {
	Number int
	Title  string
	URL    string
}

type Notifier interface {
	Send(ctx context.Context, alert Alert) error
}

func FormatBody(a Alert) string {
	var b strings.Builder
	b.WriteString(a.Title + "\n")
	if a.Repo != "" {
		b.WriteString("仓库: " + a.Repo + "\n")
	}
	if a.PRNumber > 0 {
		if a.PRURL != "" {
			fmt.Fprintf(&b, "PR: #%d %s\n", a.PRNumber, a.PRURL)
		} else {
			fmt.Fprintf(&b, "PR: #%d\n", a.PRNumber)
		}
	}
	if a.RunURL != "" {
		b.WriteString("Run: " + a.RunURL + "\n")
	}
	if len(a.Failures) > 0 {
		b.WriteString("\n失败项:\n")
		for _, f := range a.Failures {
			b.WriteString("  - " + f + "\n")
		}
	}
	if len(a.Issues) > 0 {
		b.WriteString("\n匹配 Issue:\n")
		for _, iss := range a.Issues {
			if iss.URL != "" {
				fmt.Fprintf(&b, "  - #%d %s %s\n", iss.Number, iss.Title, iss.URL)
			} else {
				fmt.Fprintf(&b, "  - #%d %s\n", iss.Number, iss.Title)
			}
		}
	}
	return b.String()
}

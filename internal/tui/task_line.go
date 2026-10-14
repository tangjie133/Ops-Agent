package tui

import (
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// FormatTaskLine 渲染一条 v1 任务，供左侧列表显示阶段和停住原因。
func FormatTaskLine(tk task.Task) string {
	line := fmt.Sprintf("%s #%d %s", tk.SourceKind, tk.SourceNumber, tk.Status)
	if tk.StopReason != "" {
		line += " " + tk.StopReason
	}
	if tk.PRURL != "" {
		line += " " + tk.PRURL
	}
	return line
}

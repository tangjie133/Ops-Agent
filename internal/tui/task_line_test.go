package tui

import (
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestFormatTaskLineShowsStopReason(t *testing.T) {
	line := FormatTaskLine(task.Task{
		SourceKind: task.SourceKindIssue, SourceNumber: 45,
		Status: task.StatusNeedsManualIntervention, StopReason: "连续 3 次测试失败",
	})
	if !strings.Contains(line, "#45") || !strings.Contains(line, "连续 3 次测试失败") {
		t.Fatalf("line=%q", line)
	}
}

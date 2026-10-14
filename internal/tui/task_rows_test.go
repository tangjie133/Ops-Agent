package tui

import (
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestSortTasksUnfinishedFirst(t *testing.T) {
	in := []task.Task{
		{SourceNumber: 1, Status: task.StatusDone},
		{SourceNumber: 2, Status: task.StatusExecuting},
		{SourceNumber: 3, Status: task.StatusCancelled},
	}
	got := SortTasksForView(in)
	if got[0].SourceNumber != 2 || got[1].SourceNumber != 1 || got[2].SourceNumber != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestSelectedTaskRows(t *testing.T) {
	rows := TaskRows(task.Task{
		Repo: "o/r", SourceNumber: 46, Status: task.StatusNeedsManualIntervention,
		StopReason: "连续 3 次测试失败",
	}, true, []string{"a", "b", "c", "d"})
	if len(rows) != 6 {
		t.Fatalf("rows=%v", rows)
	}
	if !strings.Contains(rows[0], "o/r#46") || !strings.Contains(rows[2], "b") || !strings.Contains(rows[2], "c") || strings.Contains(rows[2], "a") {
		t.Fatalf("rows=%v", rows)
	}
	if !strings.Contains(rows[3], "连续 3 次测试失败") || !strings.Contains(rows[4], "评论：—") || !strings.Contains(rows[5], "PR：—") {
		t.Fatalf("rows=%v", rows)
	}
}

func TestSelectedTaskRowsCommentOnly(t *testing.T) {
	rows := TaskRows(task.Task{Repo: "o/r", SourceNumber: 51, Status: task.StatusDone, CommentPosted: true}, true, nil)
	if !strings.Contains(rows[4], "评论：已发出") || !strings.Contains(rows[5], "PR：—") {
		t.Fatalf("%v", rows)
	}
}

func TestSelectedTaskRowsDraftPR(t *testing.T) {
	rows := TaskRows(task.Task{
		Repo: "o/r", SourceNumber: 2, Status: task.StatusDone,
		CommentPosted: true, PRURL: "https://github.com/o/r/pull/9",
	}, true, nil)
	if !strings.Contains(rows[4], "评论：已发出") || !strings.Contains(rows[5], "https://github.com/o/r/pull/9") {
		t.Fatalf("%v", rows)
	}
}

func TestSelectedTaskRowsLegacyCommentSent(t *testing.T) {
	rows := TaskRows(task.Task{Repo: "o/r", SourceNumber: 51, Status: task.StatusDone, PRURL: "已发出"}, true, nil)
	if !strings.Contains(rows[4], "评论：已发出") || !strings.Contains(rows[5], "PR：—") {
		t.Fatalf("%v", rows)
	}
}

func TestUnselectedTaskIsOneLine(t *testing.T) {
	rows := TaskRows(task.Task{Repo: "o/r", SourceNumber: 45, Status: task.StatusExecuting}, false, []string{"log"})
	if len(rows) != 1 || !strings.Contains(rows[0], "o/r#45") || !strings.Contains(rows[0], string(task.StatusExecuting)) {
		t.Fatalf("rows=%v", rows)
	}
}

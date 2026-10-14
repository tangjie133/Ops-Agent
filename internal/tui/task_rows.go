package tui

import (
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// SortTasksForView 把未完成任务排在完成和取消之前。
func SortTasksForView(in []task.Task) []task.Task {
	out := append([]task.Task(nil), in...)
	rank := func(st task.Status) int {
		if st == task.StatusDone || st == task.StatusCancelled {
			return 1
		}
		return 0
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if rank(out[j].Status) < rank(out[i].Status) || (rank(out[j].Status) == rank(out[i].Status) && out[j].SourceNumber < out[i].SourceNumber) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// TaskRows 渲染一条任务。未选中只有一行，选中再展开阶段、最近日志、原因和评论。
func TaskRows(tk task.Task, selected bool, logs []string) []string {
	head := fmt.Sprintf("%s#%d %s", tk.Repo, tk.SourceNumber, tk.Status)
	if !selected {
		return []string{head}
	}
	recent := logs
	if len(recent) > 3 {
		recent = recent[len(recent)-3:]
	}
	logText := "—"
	if len(recent) > 0 {
		logText = strings.Join(recent, " | ")
	}
	reason := tk.StopReason
	if reason == "" {
		reason = "—"
	}
	comment := "—"
	if tk.CommentPosted || tk.PRURL == "已发出" {
		comment = "已发出"
	}
	pr := "—"
	if strings.HasPrefix(tk.PRURL, "http") {
		pr = tk.PRURL
	}
	return []string{
		head,
		"阶段：" + string(tk.Status),
		"日志：" + logText,
		"原因：" + reason,
		"评论：" + comment,
		"PR：" + pr,
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestTodoFullRef(t *testing.T) {
	got := todoFullRef("tangjie133/test", 30)
	if got != "tangjie133/test#30" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatTodoEntry(t *testing.T) {
	lines := formatTodoEntry(todo.Item{
		Repo:   "tangjie133/test",
		Number: 30,
		Title:  "技术支持",
		Status: todo.StatusInTodo,
	}, 40, true, 0)
	if len(lines) != 2 {
		t.Fatalf("lines=%v", lines)
	}
	if !strings.Contains(lines[0], "tangjie133/test#30") {
		t.Fatalf("head=%q", lines[0])
	}
	if !strings.Contains(lines[1], "技术支持") {
		t.Fatalf("body=%q", lines[1])
	}
	if lines[0][0] != '>' {
		t.Fatalf("selected marker missing: %q", lines[0])
	}
}

func TestAnalyzingSpinner(t *testing.T) {
	a := analyzingSpinner(0)
	b := analyzingSpinner(1)
	if a == b {
		t.Fatalf("spinner should animate: %q %q", a, b)
	}
	lines := formatTodoEntry(todo.Item{
		Repo:   "o/r",
		Number: 1,
		Title:  "bug",
		Status: todo.StatusAnalyzing,
	}, 30, false, 0)
	if !strings.Contains(lines[0], a) {
		t.Fatalf("head=%q want spinner %q", lines[0], a)
	}
}

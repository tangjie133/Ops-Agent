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
	}, 40, true)
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

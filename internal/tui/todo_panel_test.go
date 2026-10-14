package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestTodoFullRef(t *testing.T) {
	got := todoFullRef("tangjie133/test", 30)
	if got != "tangjie133/test#30" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatTodoEntry(t *testing.T) {
	lines := formatTodoEntry(task.Task{
		Repo:         "tangjie133/test",
		SourceNumber: 30,
		Title:        "技术支持",
		Status:       task.StatusCreated,
	}, 40, true, 0)
	if len(lines) != 6 {
		t.Fatalf("lines=%v", lines)
	}
	if !strings.Contains(lines[0], "tangjie133/test#30") {
		t.Fatalf("head=%q", lines[0])
	}
	if !strings.Contains(lines[1], "阶段") {
		t.Fatalf("body=%q", lines[1])
	}
	if lines[0][0] != '>' {
		t.Fatalf("selected marker missing: %q", lines[0])
	}
}

func TestEnsureTodoSelectionKeepsAnchorAfterReorder(t *testing.T) {
	store, err := task.Load(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Hour)
	for i, repo := range []string{"o/a", "o/b", "o/c"} {
		if _, _, err := store.Ensure(task.Task{
			Repo: repo, Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue,
			SourceNumber: i + 1, Title: repo, CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}

	m := Model{tasks: store, todoSel: 2}
	m.captureTodoAnchor()
	if m.todoAnchorRepo != "o/c" || m.todoAnchorNum != 3 {
		t.Fatalf("anchor=%s#%d", m.todoAnchorRepo, m.todoAnchorNum)
	}
	m.ensureTodoSelection()
	if m.todoSel != 2 {
		t.Fatalf("todoSel=%d want 2", m.todoSel)
	}
	active := m.activeTodos()
	if active[m.todoSel].SourceNumber != 3 {
		t.Fatalf("selected #%d want #3", active[m.todoSel].SourceNumber)
	}
}

func TestAnalyzingSpinner(t *testing.T) {
	a := analyzingSpinner(0)
	b := analyzingSpinner(1)
	if a == b {
		t.Fatalf("spinner should animate: %q %q", a, b)
	}
	lines := formatTodoEntry(task.Task{
		Repo:         "o/r",
		SourceNumber: 1,
		Title:        "bug",
		Status:       task.StatusExecuting,
	}, 30, false, 0)
	if !strings.Contains(lines[0], "o/r#1") || len(lines) != 1 {
		t.Fatalf("head=%q", lines)
	}
}

func TestTodoListWrapsSelection(t *testing.T) {
	store, err := task.Load(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	for i, repo := range []string{"o/a", "o/b", "o/c"} {
		if _, _, err := store.Ensure(task.Task{
			Repo: repo, Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue,
			SourceNumber: i + 1, Title: repo,
		}); err != nil {
			t.Fatal(err)
		}
	}
	m := Model{tasks: store, todoSel: 2}
	m.todoDown()
	if m.todoSel != 0 {
		t.Fatalf("down from last: todoSel=%d want 0", m.todoSel)
	}
	m.todoUp()
	if m.todoSel != 2 {
		t.Fatalf("up from first: todoSel=%d want 2", m.todoSel)
	}
}

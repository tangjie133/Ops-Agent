package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/libtest"
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

func TestEnsureTodoSelectionKeepsAnchorAfterReorder(t *testing.T) {
	store, err := todo.Load(t.TempDir() + "/todo.json")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Hour)
	items := []todo.Item{
		{Repo: "o/a", Number: 1, Title: "A", Status: todo.StatusInTodo, CreatedAt: base, UpdatedAt: base},
		{Repo: "o/b", Number: 2, Title: "B", Status: todo.StatusInTodo, CreatedAt: base.Add(time.Minute), UpdatedAt: base.Add(time.Minute)},
		{Repo: "o/c", Number: 3, Title: "C", Status: todo.StatusInTodo, CreatedAt: base.Add(2 * time.Minute), UpdatedAt: base.Add(2 * time.Minute)},
	}
	for _, it := range items {
		if err := store.Upsert(it); err != nil {
			t.Fatal(err)
		}
	}

	m := Model{store: store, todoSel: 2}
	m.captureTodoAnchor()
	if m.todoAnchorRepo != "o/c" || m.todoAnchorNum != 3 {
		t.Fatalf("anchor=%s#%d", m.todoAnchorRepo, m.todoAnchorNum)
	}

	// 分析只更新 UpdatedAt，列表仍按 CreatedAt 入队顺序，位置不变。
	if err := store.Transition("o/c", 3, todo.StatusReady); err != nil {
		t.Fatal(err)
	}
	m.ensureTodoSelection()
	if m.todoSel != 2 {
		t.Fatalf("todoSel=%d want 2 (order unchanged)", m.todoSel)
	}
	active := m.activeTodos()
	if active[m.todoSel].Number != 3 {
		t.Fatalf("selected #%d want #3", active[m.todoSel].Number)
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

func TestTestListWrapsSelection(t *testing.T) {
	dir := t.TempDir()
	store, err := libtest.Load(dir + "/libtest.json")
	if err != nil {
		t.Fatal(err)
	}
	for i, repo := range []string{"o/a", "o/b", "o/c"} {
		if err := store.Upsert(libtest.Item{Repo: repo, Ref: "main", Title: repo, Status: libtest.StatusPending}); err != nil {
			t.Fatal(err)
		}
		_ = i
	}
	m := Model{libTestStore: store, testSel: 2}
	m.testDown()
	if m.testSel != 0 {
		t.Fatalf("down from last: testSel=%d want 0", m.testSel)
	}
	m.testUp()
	if m.testSel != 2 {
		t.Fatalf("up from first: testSel=%d want 2", m.testSel)
	}
}

func TestTodoListWrapsSelection(t *testing.T) {
	store, err := todo.Load(t.TempDir() + "/todo.json")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	for i, repo := range []string{"o/a", "o/b", "o/c"} {
		if err := store.Upsert(todo.Item{
			Repo: repo, Number: i + 1, Title: repo, Status: todo.StatusInTodo,
			CreatedAt: base, UpdatedAt: base,
		}); err != nil {
			t.Fatal(err)
		}
	}
	m := Model{store: store, todoSel: 2}
	m.todoDown()
	if m.todoSel != 0 {
		t.Fatalf("down from last: todoSel=%d want 0", m.todoSel)
	}
	m.todoUp()
	if m.todoSel != 2 {
		t.Fatalf("up from first: todoSel=%d want 2", m.todoSel)
	}
}

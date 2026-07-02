package tui

import (
	"os"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func TestHandleRefreshTickMarksDirtyWhenStoreReloads(t *testing.T) {
	dir := t.TempDir()
	store, err := todo.Load(dir + "/todo.json")
	if err != nil {
		t.Fatal(err)
	}
	libStore, err := libtest.Load(dir + "/libtest.json")
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeSemi)
	cfg.IssueAutomation.AutoAnalyze = true

	m := &Model{
		cfg:          cfg,
		store:        store,
		libTestStore: libStore,
		aiOK:         true,
		width:        120,
		height:       40,
	}
	m.storeCachedView("cached")
	if _, ok := m.tryCachedView(); !ok {
		t.Fatal("expected warm cache before refresh tick")
	}

	// 外部进程写入 todo.json，模拟 webhook/worker 更新待办。
	time.Sleep(10 * time.Millisecond)
	payload := `[{"repo":"org/repo","number":2,"title":"external","status":"in_todo","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
	if err := os.WriteFile(dir+"/todo.json", []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = m.handleRefreshTick()

	if _, ok := m.tryCachedView(); ok {
		t.Fatal("expected cache miss after store reload on refresh tick")
	}
}

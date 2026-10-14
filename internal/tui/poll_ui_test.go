package tui

import (
	"os"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func TestHandleRefreshTickMarksDirtyWhenStoreReloads(t *testing.T) {
	dir := t.TempDir()
	store, err := task.Load(dir + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeSemi)
	cfg.IssueAutomation.AutoAnalyze = true

	m := &Model{
		cfg:    cfg,
		tasks:  store,
		aiOK:   true,
		width:  120,
		height: 40,
	}
	m.storeCachedView("cached")
	if _, ok := m.tryCachedView(); !ok {
		t.Fatal("expected warm cache before refresh tick")
	}

	time.Sleep(20 * time.Millisecond)
	payload := `{"tasks":[{"id":"t1","key":"org/repo|issue_to_pr|issue|2","type":"issue_to_pr","status":"created","repo":"org/repo","source_kind":"issue","source_number":2,"title":"external"}],"runs":[]}`
	if err := os.WriteFile(dir+"/tasks.json", []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = m.handleRefreshTick()

	if _, ok := m.tryCachedView(); ok {
		t.Fatal("expected cache miss after store reload on refresh tick")
	}
}

package libtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/repovalidate"
)

func TestCheckDemos(t *testing.T) {
	dir := t.TempDir()
	ex := filepath.Join(dir, "examples", "demo1")
	if err := os.MkdirAll(ex, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ex, "demo1.ino"), []byte("void setup() {}\nvoid loop() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.LibTestConfig{MinDemos: 1, DemoDir: "examples"}
	r := CheckDemos(dir, cfg, nil)
	if !r.OK {
		t.Fatalf("%+v", r.Failures)
	}
}

func TestCheckDemosMissing(t *testing.T) {
	dir := t.TempDir()
	r := CheckDemos(dir, config.LibTestConfig{MinDemos: 1}, &repovalidate.Standard{MinDemos: 1})
	if r.OK {
		t.Fatal("expected fail")
	}
}

func TestStoreUpsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(Item{Repo: "o/r", Ref: "main", Title: "push"}); err != nil {
		t.Fatal(err)
	}
	if s.ShouldEnqueue("o/r", "main") {
		t.Fatal("pending should block re-enqueue")
	}
}

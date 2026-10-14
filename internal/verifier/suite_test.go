package verifier

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunStopsWhenNoTestCommand(t *testing.T) {
	res := Run(context.Background(), Opts{RepoPath: t.TempDir()})
	if res.OK || res.Report != "缺少验证" {
		t.Fatalf("ok=%v report=%q", res.OK, res.Report)
	}
}

func TestRunUsesInjectedRunner(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got string
	res := Run(context.Background(), Opts{
		RepoPath: dir,
		Commands: []string{"go test ./..."},
		Runner: func(ctx context.Context, d, c string) (string, error) {
			got = c
			return "ok", nil
		},
	})
	if !res.OK || got != "go test ./..." {
		t.Fatalf("ok=%v cmd=%q report=%s", res.OK, got, res.Report)
	}
}

package pr

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestResolveDiffBaseRefPrefersOrigin(t *testing.T) {
	dir := initTestRepo(t)
	ref, err := resolveDiffBaseRef(context.Background(), config.ProxyConfig{}, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if ref != "origin/main" {
		t.Fatalf("ref=%q want origin/main", ref)
	}
}

func TestCountCommitLines(t *testing.T) {
	if countCommitLines("") != 0 {
		t.Fatal("empty")
	}
	if countCommitLines("abc\n def") != 2 {
		t.Fatal("two lines")
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "t@test")
	runGit(t, dir, "config", "user.name", "t")
	writeFile(t, dir, "README", "hi")
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-m", "init")
	runGit(t, dir, "branch", "feature")
	runGit(t, dir, "checkout", "feature")
	writeFile(t, dir, "feat.txt", "x")
	runGit(t, dir, "add", "feat.txt")
	runGit(t, dir, "commit", "-m", "feat")
	runGit(t, dir, "branch", "origin/main", "main")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

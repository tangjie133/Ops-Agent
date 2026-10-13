package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "ops@example.com"},
		{"git", "config", "user.name", "ops"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add: %s", out)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %s", out)
	}
	return dir
}

func TestAllocateWorktreeIndependent(t *testing.T) {
	src := initGitRepo(t)
	root := t.TempDir()
	sess, err := AllocateFromLocal(src, root, "ops-agent/issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Dir == src {
		t.Fatal("must not reuse source dir")
	}
	if _, err := os.Stat(filepath.Join(sess.Dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Dir, "x.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "x.txt")); err == nil {
		t.Fatal("source polluted")
	}
}

func TestCleanupKeepsFailed(t *testing.T) {
	src := initGitRepo(t)
	root := t.TempDir()
	sess, err := AllocateFromLocal(src, root, "ops-agent/issue-2")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.WorkspaceConfig{KeepFailed: true, CleanupAfter: "72h"}
	if err := Cleanup(sess, true, cfg, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sess.Dir); err != nil {
		t.Fatal("failed workspace should remain")
	}
}

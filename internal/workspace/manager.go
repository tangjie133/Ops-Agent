package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// Session 是一次任务占用的隔离工作区。
type Session struct {
	Dir    string
	Branch string
	Source string
}

// AllocateFromLocal 从本地仓库创建独立 worktree；失败时回退为本地 clone。
func AllocateFromLocal(src, root, branch string) (*Session, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	name := strings.ReplaceAll(branch, "/", "-")
	dest := filepath.Join(root, name)
	cmd := exec.Command("git", "-C", src, "worktree", "add", "-b", branch, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		clone := exec.Command("git", "clone", "--local", src, dest)
		if out2, err2 := clone.CombinedOutput(); err2 != nil {
			return nil, fmt.Errorf("worktree: %s; clone: %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
		}
		co := exec.Command("git", "-C", dest, "checkout", "-b", branch)
		if out3, err3 := co.CombinedOutput(); err3 != nil {
			return nil, fmt.Errorf("checkout branch: %s", strings.TrimSpace(string(out3)))
		}
	}
	return &Session{Dir: dest, Branch: branch, Source: src}, nil
}

// Snapshot 返回工作区当前 HEAD。
func (s *Session) Snapshot() (string, error) {
	cmd := exec.Command("git", "-C", s.Dir, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("rev-parse: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Cleanup 按策略删除工作区。失败任务在 KeepFailed 为真时保留。
func Cleanup(sess *Session, failed bool, cfg config.WorkspaceConfig, now time.Time) error {
	if sess == nil {
		return nil
	}
	_ = now
	if failed && cfg.KeepFailed {
		return nil
	}
	cmd := exec.Command("git", "-C", sess.Source, "worktree", "remove", "--force", sess.Dir)
	if err := cmd.Run(); err != nil {
		if rmErr := os.RemoveAll(sess.Dir); rmErr != nil {
			return rmErr
		}
	}
	_ = exec.Command("git", "-C", sess.Source, "branch", "-D", sess.Branch).Run()
	return nil
}

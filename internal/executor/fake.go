package executor

import (
	"context"
	"os"
	"path/filepath"
)

// Fake 是测试用执行器，可写入标记文件或返回预设错误。
type Fake struct {
	Changed bool
	Err     error
	Last    Request
	Summary string
}

// Run 记录请求；Changed 为真时写入 FAKE_CHANGE.md。
func (f *Fake) Run(ctx context.Context, req Request) (Result, error) {
	f.Last = req
	if f.Err != nil {
		return Result{}, f.Err
	}
	if f.Changed && req.WorkDir != "" {
		if err := os.WriteFile(filepath.Join(req.WorkDir, "FAKE_CHANGE.md"), []byte("changed\n"), 0o644); err != nil {
			return Result{}, err
		}
	}
	summary := "fake"
	if f.Summary != "" {
		summary = f.Summary
	}
	return Result{Summary: summary, Changed: f.Changed}, nil
}

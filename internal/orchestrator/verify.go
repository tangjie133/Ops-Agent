package orchestrator

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/verifier"
)

// verifyRepo 有 go.mod 时跑 go test，否则视为缺少验证。
func verifyRepo(ctx context.Context, cfg *config.Config, dir string) verifier.Result {
	_ = cfg
	opts := verifier.Opts{RepoPath: dir}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		opts.Commands = []string{"go test ./..."}
	}
	return verifier.Run(ctx, opts)
}

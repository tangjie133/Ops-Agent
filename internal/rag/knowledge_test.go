package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestEnsureKnowledgeIndex(t *testing.T) {
	root := t.TempDir()
	cfg := config.RAGConfig{KnowledgeDir: root}
	cfg.Normalize()

	_ = os.MkdirAll(filepath.Join(root, "standards"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "standards", "t.yaml"), []byte("name: t\nrequired_files:\n  - README.md\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "datasheets"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "datasheets", "chip.md"), []byte("SD3031 CTR2 bit4 SQW enable\n"), 0o644)

	idx, err := EnsureKnowledgeIndex(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if idx == nil || len(idx.Chunks) == 0 {
		t.Fatal("expected chunks")
	}
	hits := idx.Search("SD3031 CTR2", 3)
	if len(hits) == 0 {
		t.Fatal("expected search hits")
	}
}

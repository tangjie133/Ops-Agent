package repocontext

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchRepo(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("DFRobot_SD3031.cpp", "void DFRobot_SD3031::enableFrequency(eFrequency_t fr) { reg2 |= 0x21; }")
	write("DFRobot_SD3031.h", "#define SD3031_REG_CTR2 0x10")

	files := SearchRepo(context.Background(), dir, []string{"enableFrequency", "SD3031_REG_CTR2"}, 5)
	if len(files) < 2 {
		t.Fatalf("files=%v want at least cpp+h", files)
	}
}

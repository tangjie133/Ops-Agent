package scope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV1PackagesAbsent(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	banned := []string{
		"internal/libtest",
		"internal/repovalidate",
		"internal/knowledge",
		"internal/skill",
		"internal/budget",
		"internal/feedback",
		"internal/notify",
		"internal/prcheck",
		"internal/headless",
		".worktrees",
	}
	for _, rel := range banned {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", rel)
		}
	}
}

func TestMainDoesNotImportHeadless(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "cmd", "ops-agent", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "internal/headless") {
		t.Fatal("main still imports headless")
	}
}

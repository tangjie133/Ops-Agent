package headless

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPRNumberFromEventFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	if err := os.WriteFile(path, []byte(`{"pull_request":{"number":99}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_EVENT_PATH", path)
	t.Setenv("OPS_AGENT_PR_NUMBER", "")
	if n := prNumberFromEnv(); n != 99 {
		t.Fatalf("expected 99, got %d", n)
	}
}

func TestRunURLFromEnv(t *testing.T) {
	t.Setenv("GITHUB_SERVER_URL", "https://github.com")
	t.Setenv("GITHUB_REPOSITORY", "o/r")
	t.Setenv("GITHUB_RUN_ID", "12345")
	got := runURLFromEnv()
	want := "https://github.com/o/r/actions/runs/12345"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

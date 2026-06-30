package repocontext

import "testing"

func TestExtractFileRefs(t *testing.T) {
	text := `Fix bug in internal/ai/issue.go
See ` + "`cmd/ops-agent/main.go`" + ` and README.md
Also @config/config.go`
	refs := ExtractFileRefs(text)
	want := map[string]bool{
		"internal/ai/issue.go": true,
		"cmd/ops-agent/main.go": true,
		"README.md":             true,
		"config/config.go":      true,
	}
	if len(refs) < len(want) {
		t.Fatalf("refs=%v want at least %d", refs, len(want))
	}
	for _, r := range refs {
		if !want[r] {
			t.Errorf("unexpected ref %q", r)
		}
	}
}

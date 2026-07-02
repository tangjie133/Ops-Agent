package investigator

import (
	"strings"
	"testing"
)

func TestIsRAGNoResults(t *testing.T) {
	if !isRAGNoResults("no results (检查 knowledge/datasheets") {
		t.Fatal("expected rag miss")
	}
	if isRAGNoResults("--- hit 1") {
		t.Fatal("expected hit")
	}
}

func TestWebSearchQueryFromIssue(t *testing.T) {
	q := webSearchQueryFromIssue("SD3031 CTR2 register", "")
	if q == "" || !strings.Contains(q, "SD3031") {
		t.Fatalf("q=%q", q)
	}
	if !strings.Contains(q, "datasheet") {
		t.Fatalf("expected datasheet suffix: %q", q)
	}
}

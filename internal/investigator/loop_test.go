package investigator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

type stubChat struct {
	outputs []string
	n       int
}

func (s *stubChat) ChatMessages(_ context.Context, _ []Message) (string, error) {
	if s.n >= len(s.outputs) {
		return "", context.Canceled
	}
	out := s.outputs[s.n]
	s.n++
	return out, nil
}

func TestParseAction(t *testing.T) {
	raw := `{"action":"search_repo","query":"enableFrequency"}`
	a, err := ParseAction(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Action != ActionSearch || a.Query != "enableFrequency" {
		t.Fatalf("%+v", a)
	}
}

func TestLoopSearchThenReply(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.cpp"), []byte("void enableFrequency() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.InvestigatorConfig{MaxSteps: 8, SearchMaxHits: 10}
	cfg.Normalize()
	tools := NewToolbox(dir, cfg, config.RAGConfig{}, nil, config.ProxyConfig{})

	chat := &stubChat{outputs: []string{
		`{"action":"search_repo","query":"enableFrequency"}`,
		`{"action":"read_file","path":"foo.cpp","start_line":1,"end_line":5}`,
		`{"action":"reply","body":"Thanks. See foo.cpp enableFrequency."}`,
	}}

	loop := NewLoop(cfg, chat, tools, nil)
	out, err := loop.Run(context.Background(), "Issue: enableFrequency broken")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty reply")
	}
	if chat.n != 3 {
		t.Fatalf("llm calls=%d want 3", chat.n)
	}
}

func TestToolboxListDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o644)
	cfg := config.InvestigatorConfig{}
	cfg.Normalize()
	tools := NewToolbox(dir, cfg, config.RAGConfig{}, nil, config.ProxyConfig{})
	out, err := tools.Run(context.Background(), Action{Action: ActionListDir})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("expected listing")
	}
}

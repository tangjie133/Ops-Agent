package config

import "testing"

func TestRAGOnDefaults(t *testing.T) {
	var c RAGConfig
	if !c.On() {
		t.Fatal("RAG should default enabled")
	}
	if !c.ReindexOnAnalyzeOn() {
		t.Fatal("reindex_on_analyze should default true")
	}
	c.Normalize()
	if c.InjectTopK != 4 || c.SearchTopK != 8 {
		t.Fatalf("%+v", c)
	}
}

func TestRAGDisabled(t *testing.T) {
	f := false
	c := RAGConfig{Enabled: &f}
	if c.On() {
		t.Fatal("expected disabled")
	}
}

func TestKnowledgeDirDefault(t *testing.T) {
	d := KnowledgeDir(RAGConfig{})
	if d == "" {
		t.Fatal("empty knowledge dir")
	}
}

package rag

import (
	"strings"
	"testing"
)

func TestBM25Search(t *testing.T) {
	idx := newIndex("test/repo")
	idx.Chunks = []Chunk{
		{ID: "a:1-2", Source: "src/rtc.cpp", StartLine: 1, EndLine: 2, Text: "void enableFrequency() { reg2 = 0xEF; }"},
		{ID: "b:1-2", Source: "README.md", StartLine: 1, EndLine: 2, Text: "SD3031 RTC library for Arduino"},
		{ID: "c:1-2", Source: "other.go", StartLine: 1, EndLine: 2, Text: "package main func main(){}"},
	}
	idx.rebuildStats()

	hits := idx.Search("SD3031 CTR2 register enableFrequency", 2)
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if !strings.Contains(hits[0].Chunk.Text, "enableFrequency") &&
		!strings.Contains(hits[0].Chunk.Text, "SD3031") {
		t.Fatalf("unexpected top hit: %q", hits[0].Chunk.Text)
	}
}

func TestChunkLinesOverlap(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	chunks := chunkLines("f.go", lines, 40, 8)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	if chunks[0].EndLine != 40 {
		t.Fatalf("first chunk end=%d", chunks[0].EndLine)
	}
}

func TestTokenize(t *testing.T) {
	toks := tokenize("CTR2_reg enableFrequency SD3031")
	if len(toks) < 3 {
		t.Fatalf("%v", toks)
	}
}

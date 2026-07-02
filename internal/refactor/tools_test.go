package refactor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestEditFileRejectsFullOverwrite(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "main.go")
	original := "package main\n\nfunc keep() {}\n\nfunc fix() { bug }\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	tb := NewToolbox(repo, config.InvestigatorConfig{}, config.RAGConfig{}, nil, config.ProxyConfig{})
	_, err := tb.editFile("main.go", "package main\n\nfunc fix() { ok }\n", "", "")
	if err == nil || !strings.Contains(err.Error(), "禁止") {
		t.Fatalf("expected reject full overwrite, got %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatal("file should be unchanged after rejected overwrite")
	}
}

func TestEditFilePatchReplacesOnce(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "main.go")
	original := "package main\n\nfunc keep() {}\n\nfunc fix() { bug }\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	tb := NewToolbox(repo, config.InvestigatorConfig{}, config.RAGConfig{}, nil, config.ProxyConfig{})
	msg, err := tb.editFile("main.go", "", "func fix() { bug }", "func fix() { ok }")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "patched") {
		t.Fatalf("msg=%q", msg)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "func keep() {}") {
		t.Fatalf("unchanged function removed:\n%s", got)
	}
	if !strings.Contains(got, "func fix() { ok }") {
		t.Fatalf("patch not applied:\n%s", got)
	}
}

func TestEditFilePatchRequiresUniqueOld(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "main.go")
	if err := os.WriteFile(path, []byte("x\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tb := NewToolbox(repo, config.InvestigatorConfig{}, config.RAGConfig{}, nil, config.ProxyConfig{})
	_, err := tb.editFile("main.go", "", "x", "y")
	if err == nil || !strings.Contains(err.Error(), "2 次") {
		t.Fatalf("expected ambiguous match error, got %v", err)
	}
}

func TestEditFileCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	tb := NewToolbox(repo, config.InvestigatorConfig{}, config.RAGConfig{}, nil, config.ProxyConfig{})
	msg, err := tb.editFile("new.go", "package newpkg\n", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "created") {
		t.Fatalf("msg=%q", msg)
	}
}

func TestValidateEditFilePatch(t *testing.T) {
	if err := (Action{Action: ActionEditFile, Path: "a.go", Old: "a", New: ""}).Validate(); err == nil {
		t.Fatal("expected old/new pair required")
	}
	if err := (Action{Action: ActionEditFile, Path: "a.go", Content: "x"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

package internalagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/executor"
)

type scriptLLM struct {
	replies []string
	n       int
}

func (s *scriptLLM) ChatMessages(ctx context.Context, msgs []ai.Message) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.n >= len(s.replies) {
		return `{"action":"reply","body":"done"}`, nil
	}
	out := s.replies[s.n]
	s.n++
	_ = msgs
	return out, nil
}

func TestAgentRepliesAfterRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("rtc lib"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := Agent{llm: &scriptLLM{replies: []string{
		`{"action":"read_file","path":"README.md"}`,
		`{"action":"reply","body":"这是 RTC 库"}`,
	}}}
	res, err := agent.Run(context.Background(), executor.Request{WorkDir: dir, Repo: "o/r", Number: 51, Title: "介绍库", Body: "简单介绍库功能"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "这是 RTC 库" {
		t.Fatalf("%+v", res)
	}
}

func TestCancelledContextDoesNotReply(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	agent := Agent{llm: &scriptLLM{replies: []string{`{"action":"reply","body":"不该发出"}`}}}
	_, err := agent.Run(ctx, executor.Request{WorkDir: dir, Repo: "o/r", Number: 1, Title: "t"})
	if err == nil {
		t.Fatal("cancelled context must fail")
	}
}

func TestAgentEditFileThenReply(t *testing.T) {
	dir := t.TempDir()
	run := exec.Command("git", "init", "-b", "main")
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	path := filepath.Join(dir, "src.cpp")
	if err := os.WriteFile(path, []byte("int n = 0;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := Agent{llm: &scriptLLM{replies: []string{
		`{"action":"edit_file","path":"src.cpp","old":"int n = 0;","new":"int n = 1;"}`,
		`{"action":"reply","body":"已修溢出"}`,
	}}}
	res, err := agent.Run(context.Background(), executor.Request{WorkDir: dir, Repo: "o/r", Number: 2, Title: "溢出"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "已修溢出" || !res.Changed {
		t.Fatalf("%+v", res)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "int n = 1;\n" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestEditFileRejectsContentOnExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tb := newToolbox(dir)
	_, err := tb.run(context.Background(), action{Action: "edit_file", Path: "a.txt", Content: "wipe"})
	if err == nil || !strings.Contains(err.Error(), "old/new") {
		t.Fatalf("%v", err)
	}
}

type emptyThenReplyLLM struct{ n int }

func (s *emptyThenReplyLLM) ChatMessages(ctx context.Context, msgs []ai.Message) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.n++
	if s.n == 1 {
		return "", fmt.Errorf("ai returned empty content")
	}
	return `{"action":"reply","body":"补上"}`, nil
}

func TestEmptyModelTurnContinues(t *testing.T) {
	dir := t.TempDir()
	res, err := Agent{llm: &emptyThenReplyLLM{}}.Run(context.Background(), executor.Request{WorkDir: dir, Repo: "o/r", Number: 2, Title: "t"})
	if err != nil || res.Summary != "补上" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestRejectsPathEscape(t *testing.T) {
	tb := newToolbox(t.TempDir())
	if _, err := tb.readFile("../secret", 0, 0); err == nil {
		t.Fatal("expected reject")
	}
	if _, err := tb.listDir("/etc"); err == nil && !strings.Contains(t.TempDir(), "skip") {
		t.Fatal("expected reject absolute path")
	}
}

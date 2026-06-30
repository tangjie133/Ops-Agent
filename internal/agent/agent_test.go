package agent

import (
	"context"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestDetectIntent(t *testing.T) {
	if detectIntent("帮我分析这条") != intentAnalyze {
		t.Fatal("analyze")
	}
	if detectIntent("发布到 github") != intentPost {
		t.Fatal("post")
	}
	if detectIntent("忽略这个") != intentDismiss {
		t.Fatal("dismiss")
	}
	if detectIntent("你好") != intentNone {
		t.Fatal("none")
	}
}

func TestParseIssueRef(t *testing.T) {
	repo, num, ok := parseIssueRef("处理 tangjie133/test#30 谢谢")
	if !ok || repo != "tangjie133/test" || num != 30 {
		t.Fatalf("got %s#%d ok=%v", repo, num, ok)
	}
}

func TestParseIssueNumber(t *testing.T) {
	n, ok := parseIssueNumber("看一下 #42")
	if !ok || n != 42 {
		t.Fatalf("got %d ok=%v", n, ok)
	}
}

func TestChatSemiFullRejectsAnalyzeIntent(t *testing.T) {
	cfg := config.Default()
	cfg.IssueAutomation.SetMode(config.ModeSemi)
	a := New(cfg, nil, nil)
	out, err := a.Chat(context.Background(), "分析一下", ChatContext{})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" || out == "分析一下" {
		t.Fatalf("expected hint, got %q", out)
	}
}

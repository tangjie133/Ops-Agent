package webhook

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestSkipOwnAutoReply(t *testing.T) {
	cfg := config.Default()
	if !skipOwnAutoReply(cfg, Comment{Body: "x\n---\n_Posted by Ops-Agent (auto)_", User: User{Login: "me", Type: "User"}}) {
		t.Fatal("expected skip footer comment")
	}
	if !skipOwnAutoReply(cfg, Comment{Body: "hi", User: User{Login: "dependabot", Type: "Bot"}}) {
		t.Fatal("expected skip bot")
	}
	if skipOwnAutoReply(cfg, Comment{Body: "need more help", User: User{Login: "alice", Type: "User"}}) {
		t.Fatal("should not skip user comment")
	}
}

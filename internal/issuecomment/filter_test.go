package issuecomment

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/github"
)

func TestSelectRecent(t *testing.T) {
	footer := "---\n_Posted by Ops-Agent (auto)_"
	comments := []github.IssueComment{
		{Author: github.User{Login: "user"}, Body: "first"},
		{Author: github.User{Login: "bot"}, Body: "auto\n" + footer},
		{Author: github.User{Login: "user"}, Body: "second"},
		{Author: github.User{Login: "user"}, Body: "third"},
		{Author: github.User{Login: "bot"}, Body: "_Posted by Ops-Agent manual"},
	}

	sel := SelectRecent(comments, 2, footer)
	if sel.Total != 5 || sel.ExcludedAgent != 2 {
		t.Fatalf("stats=%+v", sel)
	}
	if len(sel.Comments) != 2 {
		t.Fatalf("len=%d", len(sel.Comments))
	}
	if sel.Comments[0].Body != "second" || sel.Comments[1].Body != "third" {
		t.Fatalf("comments=%v", sel.Comments)
	}
}

func TestIsAgentReply(t *testing.T) {
	footer := "---\n_Posted by Ops-Agent (auto)_"
	if !IsAgentReply("x\n"+footer, footer) {
		t.Fatal("expected footer match")
	}
	if IsAgentReply("human question", footer) {
		t.Fatal("expected human comment")
	}
}

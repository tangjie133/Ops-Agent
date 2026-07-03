package refactor

import (
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/issuecomment"
)

func userCommentBodies(comments []github.IssueComment, agentFooter string) []string {
	sel := issuecomment.SelectRecent(comments, 0, agentFooter)
	bodies := make([]string, len(sel.Comments))
	for i, c := range sel.Comments {
		bodies[i] = c.Body
	}
	return bodies
}

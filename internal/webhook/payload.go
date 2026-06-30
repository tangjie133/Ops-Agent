package webhook

import "github.com/ZzedJay/Ops-Agent/internal/github"

type IssuesEvent struct {
	Action     string     `json:"action"`
	Issue      Issue      `json:"issue"`
	Repository Repository `json:"repository"`
}

type IssueCommentEvent struct {
	Action     string     `json:"action"`
	Issue      Issue      `json:"issue"`
	Repository Repository `json:"repository"`
}

type Issue struct {
	Number       int          `json:"number"`
	Title        string       `json:"title"`
	State        string       `json:"state"`
	HTMLURL      string       `json:"html_url"`
	Labels       []Label      `json:"labels"`
	Assignees    []Assignee   `json:"assignees"`
	PullRequest  *PullRequest `json:"pull_request,omitempty"`
}

type PullRequest struct {
	URL string `json:"url"`
}

type Label struct {
	Name string `json:"name"`
}

type Assignee struct {
	Login string `json:"login"`
}

type Repository struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type PullRequestEvent struct {
	Action     string       `json:"action"`
	PullRequest PullRequestPayload `json:"pull_request"`
	Repository Repository   `json:"repository"`
}

type PullRequestPayload struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

type PushEvent struct {
	Ref        string     `json:"ref"`
	Repository Repository `json:"repository"`
}

type ReleaseEvent struct {
	Action     string `json:"action"`
	Release    struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	} `json:"release"`
	Repository Repository `json:"repository"`
}

type RepositoryEvent struct {
	Action     string     `json:"action"`
	Repository Repository `json:"repository"`
}

func (i Issue) IsPullRequest() bool {
	return i.PullRequest != nil
}

func (i Issue) ToGitHubIssue() github.Issue {
	labels := make([]github.Label, len(i.Labels))
	for j, l := range i.Labels {
		labels[j] = github.Label{Name: l.Name}
	}
	assignees := make([]github.User, len(i.Assignees))
	for j, a := range i.Assignees {
		assignees[j] = github.User{Login: a.Login}
	}
	state := i.State
	if state != "" {
		state = stringsUpper(state)
	}
	return github.Issue{
		Number:    i.Number,
		Title:     i.Title,
		State:     state,
		URL:       i.HTMLURL,
		Labels:    labels,
		Assignees: assignees,
	}
}

func stringsUpper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

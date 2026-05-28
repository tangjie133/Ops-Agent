package webhook

import "github.com/ZzedJay/Ops-Agent/internal/github"

type IssuesEvent struct {
	Action     string     `json:"action"`
	Issue      Issue      `json:"issue"`
	Repository Repository `json:"repository"`
}

type Issue struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	Labels    []Label    `json:"labels"`
	Assignees []Assignee `json:"assignees"`
}

type Label struct {
	Name string `json:"name"`
}

type Assignee struct {
	Login string `json:"login"`
}

type Repository struct {
	FullName string `json:"full_name"`
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

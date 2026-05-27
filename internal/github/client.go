package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type AuthStatus struct {
	LoggedIn bool
	Host     string
	User     string
	Raw      string
}

type Issue struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Labels    []Label `json:"labels"`
	Assignees []User  `json:"assignees"`
	UpdatedAt string  `json:"updatedAt"`
	URL       string  `json:"url"`
	Body      string  `json:"body"`
	State     string  `json:"state"`
}

type Label struct {
	Name string `json:"name"`
}

type User struct {
	Login string `json:"login"`
}

type PullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	State     string `json:"state"`
	Body      string `json:"body"`
	Mergeable string `json:"mergeable"`
}

type ChecksResult struct {
	Raw string
}

type IssueListOpts struct {
	Repo   string
	Labels []string
	State  string
	Limit  int
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) AuthStatus(ctx context.Context) (*AuthStatus, error) {
	out, err := c.run(ctx, "auth", "status")
	if err != nil {
		return &AuthStatus{LoggedIn: false, Raw: strings.TrimSpace(string(out))}, nil
	}

	status := &AuthStatus{LoggedIn: true, Raw: strings.TrimSpace(string(out))}
	for _, line := range strings.Split(status.Raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Logged in to") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				status.Host = strings.TrimSuffix(parts[3], ",")
			}
		}
		if strings.HasPrefix(line, "Active account:") {
			status.User = strings.TrimSpace(strings.TrimPrefix(line, "Active account:"))
		}
	}
	return status, nil
}

func (c *Client) RepoFromCwd(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return "", fmt.Errorf("repo from cwd: %w", err)
	}
	var result struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("parse repo json: %w", err)
	}
	if result.NameWithOwner == "" {
		return "", fmt.Errorf("empty repo name")
	}
	return result.NameWithOwner, nil
}

func (c *Client) IssueList(ctx context.Context, opts IssueListOpts) ([]Issue, error) {
	args := []string{"issue", "list", "--json", "number,title,labels,assignees,updatedAt,url,state"}
	if opts.Repo != "" {
		args = append(args, "-R", opts.Repo)
	}
	if opts.State != "" {
		args = append(args, "--state", opts.State)
	}
	if opts.Limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", opts.Limit))
	}
	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse issues: %w", err)
	}
	return issues, nil
}

func (c *Client) IssueView(ctx context.Context, repo string, num int) (*Issue, error) {
	args := []string{
		"issue", "view", fmt.Sprintf("%d", num),
		"--json", "number,title,labels,assignees,updatedAt,url,body,state",
	}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parse issue: %w", err)
	}
	return &issue, nil
}

func (c *Client) IssueComment(ctx context.Context, repo string, num int, body string) error {
	args := []string{"issue", "comment", fmt.Sprintf("%d", num), "--body", body}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) PRView(ctx context.Context, repo string, num int) (*PullRequest, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", num),
		"--json", "number,title,url,state,body,mergeable",
	}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var pr PullRequest
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("parse pr: %w", err)
	}
	return &pr, nil
}

func (c *Client) PRChecks(ctx context.Context, repo string, num int) (*ChecksResult, error) {
	args := []string{"pr", "checks", fmt.Sprintf("%d", num)}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return &ChecksResult{Raw: string(out)}, err
	}
	return &ChecksResult{Raw: strings.TrimSpace(string(out))}, nil
}

func (c *Client) Available() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("gh %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

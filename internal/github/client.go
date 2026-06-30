package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

type AuthStatus struct {
	LoggedIn bool
	Host     string
	User     string
	Raw      string
}

type Issue struct {
	Number    int            `json:"number"`
	Title     string         `json:"title"`
	Labels    []Label        `json:"labels"`
	Assignees []User         `json:"assignees"`
	UpdatedAt string         `json:"updatedAt"`
	URL       string         `json:"url"`
	Body      string         `json:"body"`
	State     string         `json:"state"`
	Comments  []IssueComment `json:"comments"`
}

type Label struct {
	Name string `json:"name"`
}

type User struct {
	Login string `json:"login"`
}

type IssueComment struct {
	Author User   `json:"author"`
	Body   string `json:"body"`
}

type CheckContext struct {
	Context string `json:"context"`
	State   string `json:"state"`
}

type StatusCheckRollup struct {
	State    string         `json:"state"`
	Contexts []CheckContext `json:"contexts"`
}

type PullRequest struct {
	Number             int               `json:"number"`
	Title              string            `json:"title"`
	URL                string            `json:"url"`
	State              string            `json:"state"`
	Body               string            `json:"body"`
	Mergeable          string            `json:"mergeable"`
	MergeStateStatus   string            `json:"mergeStateStatus"`
	StatusCheckRollup  StatusCheckRollup `json:"statusCheckRollup"`
}

const prViewJSON = "number,title,url,state,body,mergeable,mergeStateStatus,statusCheckRollup"

type ChecksResult struct {
	Raw string
}

type IssueListOpts struct {
	Repo   string
	Labels []string
	State  string
	Limit  int
}

type Client struct {
	proxy config.ProxyConfig
}

func NewClient() *Client {
	return &Client{}
}

func NewClientWithProxy(proxy config.ProxyConfig) *Client {
	proxy.Normalize()
	return &Client{proxy: proxy}
}

func (c *Client) SetProxy(proxy config.ProxyConfig) {
	proxy.Normalize()
	c.proxy = proxy
}

func (c *Client) AuthStatus(ctx context.Context) (*AuthStatus, error) {
	out, err := c.run(ctx, "auth", "status")
	raw := strings.TrimSpace(string(out))
	if err != nil {
		return &AuthStatus{LoggedIn: false, Raw: raw}, nil
	}

	status := parseAuthStatusRaw(raw)
	if err != nil {
		status.LoggedIn = false
	}
	return status, nil
}

func parseAuthStatusRaw(raw string) *AuthStatus {
	status := &AuthStatus{LoggedIn: true, Raw: raw}
	for _, line := range strings.Split(status.Raw, "\n") {
		line = strings.TrimSpace(line)
		// gh 2.x: "✓ Logged in to github.com account USERNAME (keyring)"
		if strings.Contains(line, "Logged in to") && strings.Contains(line, " account ") {
			if user := extractBetween(line, " account ", " ("); user != "" {
				status.User = user
			}
			if host := extractLoggedInHost(line); host != "" {
				status.Host = host
			}
			continue
		}
		// legacy: "Logged in to github.com"
		if strings.HasPrefix(line, "Logged in to") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				status.Host = strings.TrimSuffix(parts[2], ",")
			}
		}
		// legacy: "Active account: username"
		if strings.HasPrefix(line, "Active account:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Active account:"))
			if val != "" && val != "true" && val != "false" {
				status.User = val
			}
		}
	}
	if status.Host == "" {
		status.Host = "github.com"
	}
	return status
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	s = s[i+len(start):]
	j := strings.Index(s, end)
	if j < 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:j])
}

func extractLoggedInHost(line string) string {
	const prefix = "Logged in to "
	i := strings.Index(line, prefix)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[i+len(prefix):])
	if j := strings.Index(rest, " account "); j >= 0 {
		return rest[:j]
	}
	parts := strings.Fields(rest)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
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
		"--json", "number,title,labels,assignees,updatedAt,url,body,state,comments",
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
		"--json", prViewJSON,
	}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	return c.parsePRView(outFromRun(c.run(ctx, args...)))
}

func (c *Client) PRViewCurrent(ctx context.Context, repo string) (*PullRequest, error) {
	args := []string{"pr", "view", "--json", prViewJSON}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	return c.parsePRView(outFromRun(c.run(ctx, args...)))
}

func outFromRun(out []byte, err error) ([]byte, error) {
	return out, err
}

func (c *Client) parsePRView(out []byte, err error) (*PullRequest, error) {
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

// CloneRepo 浅克隆仓库到 dest（使用 gh 凭证）。
func (c *Client) CloneRepo(ctx context.Context, repo, dest string) error {
	args := []string{"repo", "clone", repo, dest, "--", "--depth", "1"}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) Available() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	netproxy.ConfigureCmd(cmd, c.proxy)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("gh %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

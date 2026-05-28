package headless

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func prNumberFromEnv() int {
	if n := os.Getenv("OPS_AGENT_PR_NUMBER"); n != "" {
		var num int
		if _, err := fmt.Sscanf(n, "%d", &num); err == nil && num > 0 {
			return num
		}
	}

	path := os.Getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	var evt struct {
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(data, &evt); err != nil {
		return 0
	}
	return evt.PullRequest.Number
}

func runURLFromEnv() string {
	server := strings.TrimSuffix(os.Getenv("GITHUB_SERVER_URL"), "/")
	repo := os.Getenv("GITHUB_REPOSITORY")
	runID := os.Getenv("GITHUB_RUN_ID")
	if server == "" || repo == "" || runID == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/actions/runs/%s", server, repo, runID)
}

func repoFromEnv() string {
	return os.Getenv("GITHUB_REPOSITORY")
}

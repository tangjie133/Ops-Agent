package refactor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	ActionSearch       = "search_repo"
	ActionRead         = "read_file"
	ActionListDir      = "list_dir"
	ActionFetchURL     = "fetch_url"
	ActionWebSearch    = "web_search"
	ActionRAGSearch    = "rag_search"
	ActionRepoValidate = "repo_validate"
	ActionEditFile     = "edit_file"
	ActionRunCmd       = "run_cmd"
	ActionDone         = "done"
)

// Action 模型返回的结构化动作（JSON）。
type Action struct {
	Action    string `json:"action"`
	Query     string `json:"query,omitempty"`
	Path      string `json:"path,omitempty"`
	URL       string `json:"url,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Body      string `json:"body,omitempty"`
	Content   string `json:"content,omitempty"`
	Old       string `json:"old,omitempty"` // edit_file：待替换片段（须与 read_file 一致）
	New       string `json:"new,omitempty"` // edit_file：替换后片段
	Command   string `json:"command,omitempty"`
}

var jsonBlockRE = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// ParseAction 从模型输出解析 JSON action。
func ParseAction(raw string) (Action, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Action{}, fmt.Errorf("empty model output")
	}

	candidates := []string{raw}
	if m := jsonBlockRE.FindStringSubmatch(raw); len(m) > 1 {
		candidates = append([]string{m[1]}, candidates...)
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			candidates = append(candidates, raw[i:j+1])
		}
	}

	var lastErr error
	for _, c := range candidates {
		var a Action
		if err := json.Unmarshal([]byte(c), &a); err != nil {
			lastErr = err
			continue
		}
		a.Action = strings.TrimSpace(strings.ToLower(a.Action))
		if a.Action == "" {
			lastErr = fmt.Errorf("missing action field")
			continue
		}
		return a, nil
	}
	return Action{}, fmt.Errorf("parse action json: %w", lastErr)
}

func (a Action) Validate() error {
	switch a.Action {
	case ActionSearch:
		if strings.TrimSpace(a.Query) == "" {
			return fmt.Errorf("search_repo requires query")
		}
	case ActionRead:
		if strings.TrimSpace(a.Path) == "" {
			return fmt.Errorf("read_file requires path")
		}
	case ActionListDir, ActionRepoValidate:
	case ActionFetchURL:
		if strings.TrimSpace(a.URL) == "" {
			return fmt.Errorf("fetch_url requires url")
		}
	case ActionWebSearch, ActionRAGSearch:
		if strings.TrimSpace(a.Query) == "" {
			return fmt.Errorf("%s requires query", a.Action)
		}
	case ActionEditFile:
		if strings.TrimSpace(a.Path) == "" {
			return fmt.Errorf("edit_file requires path")
		}
		hasPatch := a.Old != "" || a.New != ""
		if hasPatch {
			if a.Old == "" || a.New == "" {
				return fmt.Errorf("edit_file: old 与 new 必须同时提供")
			}
			return nil
		}
		if a.Content == "" {
			return fmt.Errorf("edit_file: 新建文件用 content；修改已有文件用 old/new 局部替换")
		}
	case ActionRunCmd:
		if strings.TrimSpace(a.Command) == "" {
			return fmt.Errorf("run_cmd requires command")
		}
	case ActionDone:
		if strings.TrimSpace(a.Body) == "" {
			return fmt.Errorf("done requires body")
		}
	default:
		return fmt.Errorf("unknown action %q", a.Action)
	}
	return nil
}

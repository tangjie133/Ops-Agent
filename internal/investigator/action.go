package investigator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	ActionSearch  = "search_repo"
	ActionRead    = "read_file"
	ActionListDir = "list_dir"
	ActionReply   = "reply"
)

// Action 模型返回的结构化动作（JSON）。
type Action struct {
	Action    string `json:"action"`
	Query     string `json:"query,omitempty"`
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Body      string `json:"body,omitempty"`
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
	case ActionListDir:
		// path optional (root)
	case ActionReply:
		if strings.TrimSpace(a.Body) == "" {
			return fmt.Errorf("reply requires body")
		}
	default:
		return fmt.Errorf("unknown action %q", a.Action)
	}
	return nil
}

// Message 多轮 LLM 消息。
type Message struct {
	Role    string
	Content string
}

type LLM interface {
	ChatMessages(ctx context.Context, msgs []Message) (string, error)
}

// StepEvent 供 TUI/日志订阅 Agent 进度。
type StepEvent struct {
	Step        int
	MaxSteps    int
	Action      string
	Detail      string
	Observation string
}

type StepObserver func(StepEvent)

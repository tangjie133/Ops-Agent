package investigator

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// Loop 多轮 Agent 主循环。
type Loop struct {
	cfg      config.InvestigatorConfig
	client   LLM
	tools    *Toolbox
	observer StepObserver
}

func NewLoop(cfg config.InvestigatorConfig, client LLM, tools *Toolbox, observer StepObserver) *Loop {
	cfg.Normalize()
	return &Loop{cfg: cfg, client: client, tools: tools, observer: observer}
}

func (l *Loop) Run(ctx context.Context, issuePrompt string) (string, error) {
	msgs := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: issuePrompt},
	}

	toolErrors := 0
	contextUsed := len(issuePrompt)

	for step := 1; step <= l.cfg.MaxSteps; step++ {
		raw, err := l.client.ChatMessages(ctx, msgs)
		if err != nil {
			return "", err
		}

		action, err := ParseAction(raw)
		if err != nil {
			toolErrors++
			msgs = append(msgs,
				Message{Role: "assistant", Content: raw},
				Message{Role: "user", Content: parseErrorPrompt},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				return l.forceReply(ctx, msgs)
			}
			continue
		}

		if action.Action == ActionReply {
			if err := action.Validate(); err != nil {
				return "", err
			}
			l.emit(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: ActionReply, Detail: "done"})
			return strings.TrimSpace(action.Body), nil
		}

		if err := action.Validate(); err != nil {
			toolErrors++
			msgs = append(msgs,
				Message{Role: "assistant", Content: raw},
				Message{Role: "user", Content: "tool error: " + err.Error() + "\n" + parseErrorPrompt},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				return l.forceReply(ctx, msgs)
			}
			continue
		}

		obs, err := l.tools.Run(ctx, action)
		if err != nil {
			toolErrors++
			l.emit(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: action.Action, Detail: err.Error()})
			msgs = append(msgs,
				Message{Role: "assistant", Content: raw},
				Message{Role: "user", Content: fmt.Sprintf("tool error: %v\n%s", err, parseErrorPrompt)},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				return l.forceReply(ctx, msgs)
			}
			continue
		}

		toolErrors = 0
		l.emit(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: action.Action, Detail: actionDetail(action), Observation: truncateObs(obs, 200)})

		toolMsg := fmt.Sprintf("tool result (%s):\n%s", action.Action, obs)
		contextUsed += len(raw) + len(toolMsg)
		msgs = append(msgs,
			Message{Role: "assistant", Content: raw},
			Message{Role: "user", Content: toolMsg},
		)

		if contextUsed > l.cfg.TotalContextBytes {
			msgs = compressMessages(msgs, l.cfg.TotalContextBytes)
			contextUsed = estimateBytes(msgs)
		}

		if step == l.cfg.MaxSteps {
			return l.forceReply(ctx, msgs)
		}
	}

	return "", fmt.Errorf("investigator: exhausted steps without reply")
}

func (l *Loop) forceReply(ctx context.Context, msgs []Message) (string, error) {
	msgs = append(msgs, Message{Role: "user", Content: forceReplyPrompt})
	raw, err := l.client.ChatMessages(ctx, msgs)
	if err != nil {
		return "", err
	}
	action, err := ParseAction(raw)
	if err != nil {
		return strings.TrimSpace(raw), nil
	}
	if action.Action == ActionReply && strings.TrimSpace(action.Body) != "" {
		return strings.TrimSpace(action.Body), nil
	}
	return strings.TrimSpace(raw), nil
}

func (l *Loop) emit(ev StepEvent) {
	if l.observer != nil {
		l.observer(ev)
	}
}

func actionDetail(a Action) string {
	switch a.Action {
	case ActionSearch:
		return a.Query
	case ActionRead:
		return fmt.Sprintf("%s:%d-%d", a.Path, a.StartLine, a.EndLine)
	case ActionListDir:
		return a.Path
	default:
		return a.Action
	}
}

func truncateObs(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func estimateBytes(msgs []Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content)
	}
	return n
}

// compressMessages 保留 system、初始 user，以及最近若干轮 tool 对话。
func compressMessages(msgs []Message, budget int) []Message {
	if len(msgs) <= 4 || budget <= 0 {
		return msgs
	}
	out := []Message{msgs[0], msgs[1]}
	tail := msgs[2:]
	for len(tail) > 6 {
		tail = tail[2:]
	}
	out = append(out, tail...)
	for estimateBytes(out) > budget && len(out) > 4 {
		if len(out) > 5 {
			out = append(out[:2], out[4:]...)
		} else {
			break
		}
	}
	return out
}

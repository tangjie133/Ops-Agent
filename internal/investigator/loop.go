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
	log      Logger
}

func NewLoop(cfg config.InvestigatorConfig, client LLM, tools *Toolbox, observer StepObserver) *Loop {
	cfg.Normalize()
	return &Loop{cfg: cfg, client: client, tools: tools, observer: observer}
}

func (l *Loop) SetLogger(log Logger) {
	l.log = log
}

func (l *Loop) Run(ctx context.Context, issuePrompt string) (string, error) {
	msgs := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: issuePrompt},
	}

	toolErrors := 0
	contextUsed := len(issuePrompt)
	logf(l.log, "Investigator Agent 循环开始 (max_steps=%d)", l.cfg.MaxSteps)

	for step := 1; step <= l.cfg.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		raw, err := l.client.ChatMessages(ctx, msgs)
		if err != nil {
			logf(l.log, "Investigator [%d/%d] LLM 失败: %v", step, l.cfg.MaxSteps, err)
			return "", err
		}

		action, err := ParseAction(raw)
		if err != nil {
			toolErrors++
			logf(l.log, "Investigator [%d/%d] JSON 解析失败: %v · raw=%q", step, l.cfg.MaxSteps, err, truncateObs(raw, 80))
			l.emitStep(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: "parse", Err: err.Error()})
			msgs = append(msgs,
				Message{Role: "assistant", Content: raw},
				Message{Role: "user", Content: parseErrorPrompt},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				logf(l.log, "Investigator 连续解析失败，强制 reply")
				return l.forceReply(ctx, msgs)
			}
			continue
		}

		if action.Action == ActionReply {
			if err := action.Validate(); err != nil {
				return "", err
			}
			l.emitStep(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: ActionReply, Detail: "done"})
			logf(l.log, "Investigator [%d/%d] reply 完成 (%d 字符)", step, l.cfg.MaxSteps, len(action.Body))
			return strings.TrimSpace(action.Body), nil
		}

		if err := action.Validate(); err != nil {
			toolErrors++
			l.emitStep(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: action.Action, Err: err.Error()})
			msgs = append(msgs,
				Message{Role: "assistant", Content: raw},
				Message{Role: "user", Content: "tool error: " + err.Error() + "\n" + parseErrorPrompt},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				return l.forceReply(ctx, msgs)
			}
			continue
		}

		detail := actionDetail(action)
		logf(l.log, "Investigator [%d/%d] → %s %s", step, l.cfg.MaxSteps, action.Action, detail)

		obs, err := l.tools.Run(ctx, action)
		if err != nil {
			toolErrors++
			l.emitStep(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: action.Action, Detail: detail, Err: err.Error()})
			logf(l.log, "Investigator [%d/%d] %s 失败: %v", step, l.cfg.MaxSteps, action.Action, err)
			msgs = append(msgs,
				Message{Role: "assistant", Content: raw},
				Message{Role: "user", Content: fmt.Sprintf("tool error: %v\n%s", err, parseErrorPrompt)},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				logf(l.log, "Investigator 连续工具失败，强制 reply")
				return l.forceReply(ctx, msgs)
			}
			continue
		}

		toolErrors = 0
		summary := summarizeObservation(action.Action, obs)
		l.emitStep(StepEvent{Step: step, MaxSteps: l.cfg.MaxSteps, Action: action.Action, Detail: detail, Observation: summary})
		logf(l.log, "Investigator [%d/%d] %s OK · %s", step, l.cfg.MaxSteps, action.Action, summary)

		toolMsg := fmt.Sprintf("tool result (%s):\n%s", action.Action, obs)
		contextUsed += len(raw) + len(toolMsg)
		msgs = append(msgs,
			Message{Role: "assistant", Content: raw},
			Message{Role: "user", Content: toolMsg},
		)

		if contextUsed > l.cfg.TotalContextBytes {
			logf(l.log, "Investigator 上下文压缩 (%d bytes)", contextUsed)
			msgs = compressMessages(msgs, l.cfg.TotalContextBytes)
			contextUsed = estimateBytes(msgs)
		}

		if step == l.cfg.MaxSteps {
			logf(l.log, "Investigator 达到 max_steps，强制 reply")
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
		logf(l.log, "Investigator 强制 reply 非 JSON，使用原文")
		return strings.TrimSpace(raw), nil
	}
	if action.Action == ActionReply && strings.TrimSpace(action.Body) != "" {
		return strings.TrimSpace(action.Body), nil
	}
	return strings.TrimSpace(raw), nil
}

func (l *Loop) emitStep(ev StepEvent) {
	logf(l.log, formatStep(ev))
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
	case ActionFetchURL:
		return a.URL
	case ActionWebSearch:
		return a.Query
	case ActionRAGSearch:
		return a.Query
	case ActionRepoValidate:
		if q := strings.TrimSpace(a.Query); q != "" {
			return q
		}
		return "(default)"
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

package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

const chatSystemPrompt = `你是 Ops-Agent，GitHub 运维 TUI 助手。
你可以帮助用户理解待办 Issue、自动化模式、Webhook 配置。
回答简洁，使用中文。`

type Agent struct {
	cfg    *config.Config
	gh     *github.Client
	store  *todo.FileStore
	chat   aiClient
	invLog func(string)
}

func (a *Agent) SetInvestigatorLog(log func(string)) {
	a.invLog = log
}

func New(cfg *config.Config, gh *github.Client, store *todo.FileStore) *Agent {
	return &Agent{
		cfg:   cfg,
		gh:    gh,
		store: store,
		chat:  ai.NewClient(cfg.AI),
	}
}

// Chat 处理自然语言输入；manual 模式下支持聊天处理待办。
func (a *Agent) Chat(ctx context.Context, line string, cx ChatContext) (string, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil
	}

	if a.cfg.IssueAutomation.Mode == config.ModeManual {
		if out, handled, err := a.handleManualChat(ctx, line, cx); handled {
			return out, err
		}
		return a.generalChat(ctx, line, cx, manualChatSystemPrompt)
	}

	if detectIntent(line) != intentNone {
		return semiFullAnalyzeHint(), nil
	}

	return a.generalChat(ctx, line, cx, chatSystemPrompt)
}

func (a *Agent) generalChat(ctx context.Context, line string, cx ChatContext, system string) (string, error) {
	var parts []string
	if summary := a.todoSummary(); summary != "" {
		parts = append(parts, summary)
	}
	if sel := a.selectedSummary(cx.Selected); sel != "" {
		parts = append(parts, sel)
	}
	user := line
	if len(parts) > 0 {
		user = strings.Join(parts, "\n") + "\n\n用户: " + line
	}
	return a.chat.Chat(ctx, system, user)
}

func (a *Agent) todoSummary() string {
	if a.store == nil {
		return ""
	}
	active := 0
	var refs []string
	for _, it := range a.store.List() {
		switch it.Status {
		case todo.StatusDismissed, todo.StatusDone:
			continue
		}
		active++
		if len(refs) < 5 {
			refs = append(refs, fmt.Sprintf("%s#%d (%s)", it.Repo, it.Number, it.Status))
		}
	}
	if active == 0 {
		return "当前无活跃待办。"
	}
	return fmt.Sprintf("当前待办 %d 条: %s", active, strings.Join(refs, ", "))
}

type aiClient interface {
	Chat(ctx context.Context, system, user string) (string, error)
}

// 确保 *ai.Client 满足 aiClient
var _ aiClient = (*ai.Client)(nil)

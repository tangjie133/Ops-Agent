package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

const chatSystemPrompt = "你是 Ops-Agent 的操作对话。只根据给出的任务状态回答。不要声称已经修改代码、打开 PR、合并、关闭 Issue 或 force push。这些动作只有用户用明确指令才会执行。"

func (m *Model) answerChat(line string) tea.Cmd {
	if m.cfg == nil {
		return func() tea.Msg {
			return commandDoneMsg{output: "没有配置，无法问模型。"}
		}
	}
	cfg := m.cfg.AI
	status := m.chatStatusText()
	ctx := m.bgCtx()
	return func() tea.Msg {
		out, err := ai.NewClient(cfg).Chat(ctx, chatSystemPrompt, status+"\n\n用户："+line)
		if err != nil {
			return commandDoneMsg{output: "模型没有回答: " + err.Error()}
		}
		return commandDoneMsg{output: out}
	}
}

func (m *Model) chatStatusText() string {
	var b strings.Builder
	aiState := "不可达"
	if m.aiOK {
		aiState = "可达"
	}
	wh := "关"
	modelName := ""
	repo := m.repo
	if m.cfg != nil {
		modelName = m.cfg.AI.Model
		if m.cfg.Webhook.Enabled {
			wh = "开"
		}
	}
	active := 0
	if m.tasks != nil {
		active = m.tasks.ActiveCount()
	}
	fmt.Fprintf(&b, "模型 %s（%s）。webhook %s。仓库 %s。未完成任务 %d。\n", modelName, aiState, wh, repo, active)
	var items []task.Task
	if m.tasks != nil {
		items = SortTasksForView(m.tasks.List())
	}
	for _, tk := range items {
		reason := tk.StopReason
		if reason == "" {
			reason = "—"
		}
		comment := "—"
		if tk.PRURL != "" {
			comment = "已发出"
		}
		fmt.Fprintf(&b, "%s#%d 阶段 %s 原因 %s 评论 %s 标题 %s\n", tk.Repo, tk.SourceNumber, tk.Status, reason, comment, tk.Title)
	}
	return b.String()
}

func (m *Model) applyChatAction(a *ChatAction) string {
	if a == nil {
		return ""
	}
	switch a.Kind {
	case "stop":
		return m.setTaskStopped(a, task.StatusNeedsManualIntervention, "用户停止")
	case "skip":
		return m.setTaskStopped(a, task.StatusCancelled, "用户跳过")
	case "retry":
		return m.requeueTask(a)
	case "set_model":
		m.cfg.AI.Model = a.Ref
		return m.saveConfig("模型 " + a.Ref)
	case "set_webhook":
		m.cfg.Webhook.Enabled = a.Ref == "开"
		return m.saveConfig("webhook " + a.Ref)
	case "set_proxy":
		m.cfg.Proxy.HTTPSProxy = a.Ref
		m.cfg.Proxy.Enabled = a.Ref != ""
		label := a.Ref
		if label == "" {
			label = "关"
		}
		return m.saveConfig("代理 " + label)
	case "merge":
		if err := m.gh.PRMerge(m.bgCtx(), a.Repo, a.Number); err != nil {
			return fmt.Sprintf("合并 %s#%d 失败: %v", a.Repo, a.Number, err)
		}
		return fmt.Sprintf("已合并 %s#%d", a.Repo, a.Number)
	case "close_issue":
		if err := m.gh.IssueClose(m.bgCtx(), a.Repo, a.Number); err != nil {
			return fmt.Sprintf("关闭 %s#%d 失败: %v", a.Repo, a.Number, err)
		}
		return fmt.Sprintf("已关闭 %s#%d", a.Repo, a.Number)
	case "force_push":
		if err := m.gh.ForcePush(m.bgCtx(), a.Ref); err != nil {
			return fmt.Sprintf("force push %s %s 失败: %v", a.Repo, a.Ref, err)
		}
		return fmt.Sprintf("已 force push %s %s", a.Repo, a.Ref)
	default:
		return "未执行"
	}
}

func (m *Model) saveConfig(what string) string {
	if _, err := config.Save(m.cfg); err != nil {
		return what + " 保存失败: " + err.Error()
	}
	return "已保存 " + what
}

func (m *Model) setTaskStopped(a *ChatAction, status task.Status, reason string) string {
	tk, ok := m.findChatTask(a.Repo, a.Number)
	if !ok {
		return fmt.Sprintf("没有任务 %s#%d", a.Repo, a.Number)
	}
	tk.Status = status
	tk.StopReason = reason
	tk.ActiveRunID = ""
	if err := m.tasks.Update(tk); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s %s#%d", reason, a.Repo, a.Number)
}

func (m *Model) requeueTask(a *ChatAction) string {
	tk, ok := m.findChatTask(a.Repo, a.Number)
	if !ok {
		return fmt.Sprintf("没有任务 %s#%d", a.Repo, a.Number)
	}
	tk.Status = task.StatusCreated
	tk.StopReason = ""
	tk.ActiveRunID = ""
	if err := m.tasks.Update(tk); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("重试 %s#%d", a.Repo, a.Number)
}

func (m *Model) findChatTask(repo string, number int) (task.Task, bool) {
	if m.tasks == nil {
		return task.Task{}, false
	}
	if tk, ok := m.tasks.GetByKey(task.MakeTaskKey(repo, task.TypeIssueToPR, task.SourceKindIssue, number)); ok {
		return tk, true
	}
	return m.tasks.GetByKey(task.MakeTaskKey(repo, task.TypeIssueToPR, task.SourceKindPR, number))
}

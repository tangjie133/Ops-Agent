package tui

import (
	"fmt"
	"strconv"
	"strings"
)

// ChatAction 是对话解析出的一次操作。
type ChatAction struct {
	Kind   string
	Repo   string
	Number int
	Ref    string
}

// ChatReply 是写回对话的文字，以及确认后才非空的执行动作。
type ChatReply struct {
	Text    string
	Execute *ChatAction
}

// HandleChat 处理一句输入。高风险动作要等下一句「确认」。
func HandleChat(pending *ChatAction, line string) (*ChatAction, ChatReply) {
	line = strings.TrimSpace(line)
	if line == "" {
		return pending, ChatReply{}
	}
	if pending != nil {
		if line == "确认" {
			return nil, ChatReply{Text: "执行 " + describe(pending), Execute: pending}
		}
		if next := parseCommand(line); next != nil && highRisk(next.Kind) {
			return next, ChatReply{Text: "已取消 " + describe(pending) + "。改为 " + describe(next) + "。输入确认后执行。"}
		}
		return nil, ChatReply{Text: "不执行 " + describe(pending)}
	}
	action := parseCommand(line)
	if action == nil {
		if strings.Contains(line, "改代码") || strings.Contains(line, "开 PR") {
			return nil, ChatReply{Text: "自动开 draft PR 由任务队列完成。"}
		}
		return nil, ChatReply{Text: "可以查询任务状态，或说明要停止、重试、跳过、改配置。"}
	}
	if highRisk(action.Kind) {
		return action, ChatReply{Text: describe(action) + "。输入确认后执行。"}
	}
	return nil, ChatReply{Text: "执行 " + describe(action), Execute: action}
}

// chatNeedsModel 表示这句不是操作指令，要交给模型回答。
func chatNeedsModel(reply ChatReply) bool {
	return reply.Execute == nil && strings.Contains(reply.Text, "可以查询任务状态")
}

func highRisk(kind string) bool {
	switch kind {
	case "merge", "close_issue", "force_push":
		return true
	default:
		return false
	}
}

func describe(a *ChatAction) string {
	if a.Number > 0 {
		return fmt.Sprintf("%s %s#%d %s", a.Kind, a.Repo, a.Number, a.Ref)
	}
	return fmt.Sprintf("%s %s %s", a.Kind, a.Repo, a.Ref)
}

func parseCommand(line string) *ChatAction {
	switch {
	case strings.HasPrefix(line, "合并 "):
		repo, num, ok := parseRepoNum(strings.TrimPrefix(line, "合并 "))
		if !ok {
			return nil
		}
		return &ChatAction{Kind: "merge", Repo: repo, Number: num}
	case strings.HasPrefix(line, "关闭 "):
		repo, num, ok := parseRepoNum(strings.TrimPrefix(line, "关闭 "))
		if !ok {
			return nil
		}
		return &ChatAction{Kind: "close_issue", Repo: repo, Number: num}
	case strings.HasPrefix(line, "force push "):
		rest := strings.TrimPrefix(line, "force push ")
		repo, ref, ok := strings.Cut(rest, " 分支 ")
		if !ok || repo == "" || ref == "" {
			return nil
		}
		return &ChatAction{Kind: "force_push", Repo: repo, Ref: ref}
	case strings.HasPrefix(line, "停止 "):
		repo, num, ok := parseRepoNum(strings.TrimPrefix(line, "停止 "))
		if !ok {
			return nil
		}
		return &ChatAction{Kind: "stop", Repo: repo, Number: num}
	case strings.HasPrefix(line, "重试 "):
		repo, num, ok := parseRepoNum(strings.TrimPrefix(line, "重试 "))
		if !ok {
			return nil
		}
		return &ChatAction{Kind: "retry", Repo: repo, Number: num}
	case strings.HasPrefix(line, "跳过 "):
		repo, num, ok := parseRepoNum(strings.TrimPrefix(line, "跳过 "))
		if !ok {
			return nil
		}
		return &ChatAction{Kind: "skip", Repo: repo, Number: num}
	case strings.HasPrefix(line, "模型 "):
		name := strings.TrimSpace(strings.TrimPrefix(line, "模型 "))
		if name == "" {
			return nil
		}
		return &ChatAction{Kind: "set_model", Ref: name}
	case strings.HasPrefix(line, "webhook "):
		val := strings.TrimSpace(strings.TrimPrefix(line, "webhook "))
		if val != "开" && val != "关" {
			return nil
		}
		return &ChatAction{Kind: "set_webhook", Ref: val}
	case strings.HasPrefix(line, "代理 "):
		val := strings.TrimSpace(strings.TrimPrefix(line, "代理 "))
		if val == "" {
			return nil
		}
		if val == "关" {
			val = ""
		}
		return &ChatAction{Kind: "set_proxy", Ref: val}
	default:
		return nil
	}
}

func parseRepoNum(rest string) (string, int, bool) {
	rest = strings.TrimSpace(rest)
	i := strings.LastIndex(rest, "#")
	if i <= 0 || i == len(rest)-1 {
		return "", 0, false
	}
	n, err := strconv.Atoi(rest[i+1:])
	if err != nil {
		return "", 0, false
	}
	return rest[:i], n, true
}

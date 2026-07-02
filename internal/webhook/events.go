package webhook

import "fmt"

// Event 描述一次 webhook 处理结果，供 TUI 展示。
type Event struct {
	Kind   EventKind
	Repo   string
	Number int
	Title  string
	Reason string // skipped 时的原因
}

type EventKind string

const (
	EventAdded        EventKind = "added"
	EventCommentAdded EventKind = "comment_added"
	EventClosed       EventKind = "closed"
	EventReopened  EventKind = "reopened"
	EventSkipped   EventKind = "skipped"
	EventPing      EventKind = "ping"
	EventIgnored      EventKind = "ignored"
	EventLibTestQueued EventKind = "lib_test_queued"
	EventFixConfirmed  EventKind = "fix_confirmed"
)

func (e Event) Message() string {
	switch e.Kind {
	case EventAdded:
		if e.Title != "" {
			return fmt.Sprintf("Webhook: 新待办 %s#%d %s", e.Repo, e.Number, e.Title)
		}
		return fmt.Sprintf("Webhook: 新待办 %s#%d", e.Repo, e.Number)
	case EventCommentAdded:
		if e.Title != "" {
			return fmt.Sprintf("Webhook: 评论触发入队 %s#%d %s", e.Repo, e.Number, e.Title)
		}
		return fmt.Sprintf("Webhook: 评论触发入队 %s#%d", e.Repo, e.Number)
	case EventClosed:
		if e.Title != "" {
			return fmt.Sprintf("Webhook: 已从待办移除 %s#%d %s（GitHub 已关闭）", e.Repo, e.Number, e.Title)
		}
		return fmt.Sprintf("Webhook: 已从待办移除 %s#%d（GitHub 已关闭）", e.Repo, e.Number)
	case EventReopened:
		if e.Title != "" {
			return fmt.Sprintf("Webhook: 重新入队 %s#%d %s", e.Repo, e.Number, e.Title)
		}
		return fmt.Sprintf("Webhook: 重新入队 %s#%d", e.Repo, e.Number)
	case EventSkipped:
		return fmt.Sprintf("Webhook: 跳过 %s#%d (%s)", e.Repo, e.Number, e.Reason)
	case EventPing:
		return "Webhook: ping 成功，GitHub 已连通"
	case EventIgnored:
		if e.Reason != "" {
			return "Webhook: 忽略事件 (" + e.Reason + ")"
		}
		return "Webhook: 忽略未支持的事件"
	case EventLibTestQueued:
		if e.Title != "" {
			return fmt.Sprintf("Webhook: 验收入队 %s (%s) %s", e.Repo, e.Reason, e.Title)
		}
		return fmt.Sprintf("Webhook: 验收入队 %s (%s)", e.Repo, e.Reason)
	case EventFixConfirmed:
		if e.Title != "" {
			return fmt.Sprintf("Webhook: /approve-pr 已确认修库 %s#%d %s", e.Repo, e.Number, e.Title)
		}
		return fmt.Sprintf("Webhook: /approve-pr 已确认修库 %s#%d", e.Repo, e.Number)
	default:
		return "Webhook: 事件已处理"
	}
}

type OnEvent func(Event)

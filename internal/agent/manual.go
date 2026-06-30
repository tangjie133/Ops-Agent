package agent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type chatIntent int

const (
	intentNone chatIntent = iota
	intentAnalyze
	intentPost
	intentDismiss
)

var (
	reRepoIssue = regexp.MustCompile(`(?i)([a-z0-9_.-]+/[a-z0-9_.-]+)#(\d+)`)
	reHashNum   = regexp.MustCompile(`#(\d+)`)
	reBareNum   = regexp.MustCompile(`(?:^|\s)(\d{1,6})(?:\s|$|号)`)
)

func detectIntent(line string) chatIntent {
	lower := strings.ToLower(line)
	switch {
	case containsAny(lower, "发布", "发送", "post", "reply", "确认发布", "发评论"):
		return intentPost
	case containsAny(lower, "忽略", "跳过", "dismiss", "不要这条"):
		return intentDismiss
	case containsAny(lower, "分析", "处理", "生成", "写回复", "analyze", "draft", "回复"):
		return intentAnalyze
	default:
		return intentNone
	}
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// ChatContext 传入 TUI 当前选中待办等上下文。
type ChatContext struct {
	Selected *todo.Item
}

func (a *Agent) resolveTarget(line string, cx ChatContext) (*todo.Item, error) {
	if ref, num, ok := parseIssueRef(line); ok {
		if it, ok := a.store.Get(ref, num); ok {
			item := it
			return &item, nil
		}
		return nil, fmt.Errorf("待办中未找到 %s#%d", ref, num)
	}
	if num, ok := parseIssueNumber(line); ok {
		matches := a.activeByNumber(num)
		switch len(matches) {
		case 1:
			item := matches[0]
			return &item, nil
		case 0:
			return nil, fmt.Errorf("待办中未找到 #%d", num)
		default:
			var refs []string
			for _, it := range matches {
				refs = append(refs, fmt.Sprintf("%s#%d", it.Repo, it.Number))
			}
			return nil, fmt.Errorf("多个待办匹配 #%d，请指定仓库: %s", num, strings.Join(refs, ", "))
		}
	}
	if cx.Selected != nil {
		item := *cx.Selected
		return &item, nil
	}
	return nil, fmt.Errorf("请先用 j/k 选中待办，或在消息里写 owner/repo#编号")
}

func parseIssueRef(line string) (repo string, num int, ok bool) {
	if m := reRepoIssue.FindStringSubmatch(line); len(m) == 3 {
		n, err := parseInt(m[2])
		if err != nil || n <= 0 {
			return "", 0, false
		}
		return m[1], n, true
	}
	return "", 0, false
}

func parseIssueNumber(line string) (int, bool) {
	if m := reHashNum.FindStringSubmatch(line); len(m) == 2 {
		n, err := parseInt(m[1])
		if err == nil && n > 0 {
			return n, true
		}
	}
	if m := reBareNum.FindStringSubmatch(line); len(m) == 2 {
		n, err := parseInt(m[1])
		if err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func (a *Agent) activeByNumber(num int) []todo.Item {
	var out []todo.Item
	for _, it := range a.store.List() {
		if it.Number != num {
			continue
		}
		switch it.Status {
		case todo.StatusDismissed, todo.StatusDone:
			continue
		}
		out = append(out, it)
	}
	return out
}

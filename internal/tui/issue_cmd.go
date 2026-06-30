package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// parseIssueArgs 解析 /issue 参数，支持 owner/repo#n、owner/repo n、或仅编号（从待办反查仓库）。
func parseIssueArgs(parts []string, store *todo.FileStore, cwdRepo string) (repo string, num int, err string) {
	if len(parts) < 2 {
		return "", 0, "用法: /issue owner/repo#n 或 /issue owner/repo n"
	}

	rest := parts[1:]
	if strings.Contains(rest[0], "#") {
		repo, num, ok := splitRepoIssue(rest[0])
		if !ok {
			return "", 0, "无效格式，示例: /issue tangjie133/test#30"
		}
		return repo, num, ""
	}

	if len(rest) >= 2 {
		if n, e := strconv.Atoi(rest[1]); e == nil && n > 0 {
			return rest[0], n, ""
		}
	}

	if n, e := strconv.Atoi(rest[0]); e == nil && n > 0 {
		repo, ok, msg := lookupIssueRepo(store, n, cwdRepo)
		if !ok {
			return "", 0, msg
		}
		return repo, n, ""
	}

	return "", 0, "无效参数，示例: /issue tangjie133/test#30"
}

func splitRepoIssue(token string) (repo string, num int, ok bool) {
	i := strings.LastIndex(token, "#")
	if i <= 0 || i >= len(token)-1 {
		return "", 0, false
	}
	n, err := strconv.Atoi(token[i+1:])
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return token[:i], n, true
}

func lookupIssueRepo(store *todo.FileStore, num int, cwdRepo string) (repo string, ok bool, err string) {
	var active []todo.Item
	for _, it := range store.List() {
		if it.Number != num {
			continue
		}
		switch it.Status {
		case todo.StatusDismissed, todo.StatusDone:
			continue
		}
		active = append(active, it)
	}
	switch len(active) {
	case 1:
		return active[0].Repo, true, ""
	case 0:
		if cwdRepo != "" {
			return cwdRepo, true, ""
		}
		return "", false, "无法解析仓库，请使用 /issue owner/repo#n"
	default:
		var refs []string
		for _, it := range active {
			refs = append(refs, fmt.Sprintf("%s#%d", it.Repo, it.Number))
		}
		return "", false, "多个待办匹配 #" + strconv.Itoa(num) + "，请指定仓库:\n  /issue " + strings.Join(refs, "\n  /issue ")
	}
}

func formatIssueRef(repo string, num int) string {
	if repo == "" {
		return fmt.Sprintf("#%d", num)
	}
	return fmt.Sprintf("%s#%d", repo, num)
}

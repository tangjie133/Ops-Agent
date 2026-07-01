package tui

// completion.go — 斜杠命令与待办编号的 Tab 补全。

import (
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// Completion 表示一条 Tab 补全候选（整行文本 + 提示）。
type Completion struct {
	Text string // 补全后的整行输入
	Hint string // 说明文字
}

type commandSpec struct {
	name string
	hint string
	args []string
}

var slashCommands = []commandSpec{
	{name: "/help", hint: "显示帮助"},
	{name: "/status", hint: "gh 与 llama 状态"},
	{name: "/clean", hint: "清空输出区"},
	{name: "/check", hint: "PR 检测"},
	{name: "/webhook", hint: "Webhook 配置菜单"},
	{name: "/accept", hint: "验收配置（手动/自动）"},
	{name: "/model", hint: "模型配置菜单"},
	{name: "/ai", hint: "模型配置菜单"},
	{name: "/proxy", hint: "网络代理配置菜单"},
	{name: "/vpn", hint: "网络代理配置菜单"},
	{name: "/mode", hint: "模式选择菜单"},
	{name: "/issue", hint: "查看 issue"},
	{name: "/feedback", hint: "反馈（M4）"},
}

func computeCompletions(line string, todos []todo.Item) []Completion {
	if line == "" || !strings.HasPrefix(line, "/") {
		return nil
	}

	trailingSpace := strings.HasSuffix(line, " ")
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) == 0 {
		return allCommandCompletions("")
	}

	cmdToken := strings.ToLower(parts[0])

	// 还在输入命令名
	if len(parts) == 1 && !trailingSpace {
		return allCommandCompletions(cmdToken)
	}

	spec, ok := findCommand(cmdToken)
	if !ok {
		return nil
	}

	// 补全子参数
	switch spec.name {
	case "/issue":
		return issueCompletions(parts, trailingSpace, todos)
	default:
		return nil
	}
}

func allCommandCompletions(prefix string) []Completion {
	prefix = strings.ToLower(prefix)
	for _, c := range slashCommands {
		if c.name == prefix {
			return []Completion{{Text: c.name, Hint: c.hint}}
		}
	}
	var out []Completion
	for _, c := range slashCommands {
		if prefix == "" || strings.HasPrefix(c.name, prefix) {
			out = append(out, Completion{Text: c.name, Hint: c.hint})
		}
	}
	return out
}

func findCommand(token string) (commandSpec, bool) {
	for _, c := range slashCommands {
		if c.name == token {
			return c, true
		}
	}
	return commandSpec{}, false
}

func argCompletions(spec commandSpec, parts []string, trailingSpace bool) []Completion {
	var argPrefix string
	if len(parts) >= 2 {
		argPrefix = strings.ToLower(parts[1])
	} else if !trailingSpace {
		return nil
	}

	var out []Completion
	for _, arg := range spec.args {
		if argPrefix == "" || strings.HasPrefix(arg, argPrefix) {
			out = append(out, Completion{
				Text: spec.name + " " + arg,
				Hint: spec.hint,
			})
		}
	}
	return out
}

func issueCompletions(parts []string, trailingSpace bool, todos []todo.Item) []Completion {
	if len(parts) == 1 && !trailingSpace {
		return []Completion{{Text: "/issue ", Hint: "issue 编号"}}
	}
	if len(parts) >= 2 && !trailingSpace {
		prefix := parts[1]
		var out []Completion
		for _, it := range todos {
			num := fmt.Sprintf("%d", it.Number)
			ref := it.Repo + "#" + num
			if strings.HasPrefix(ref, prefix) || strings.HasPrefix(num, prefix) {
				out = append(out, Completion{
					Text: "/issue " + ref,
					Hint: truncate(it.Title, 24),
				})
			}
		}
		return out
	}
	if trailingSpace || len(parts) == 1 {
		var out []Completion
		for _, it := range todos {
			out = append(out, Completion{
				Text: "/issue " + it.Repo + "#" + fmt.Sprintf("%d", it.Number),
				Hint: truncate(it.Title, 24),
			})
		}
		if len(out) == 0 {
			return []Completion{{Text: "/issue ", Hint: "输入编号"}}
		}
		return out
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ghostSuffix 返回最佳匹配的剩余后缀（用于 inline 提示）。
func ghostSuffix(line string, completions []Completion) string {
	if len(completions) == 0 {
		return ""
	}
	best := completions[0].Text
	if strings.HasPrefix(best, line) && len(best) > len(line) {
		return best[len(line):]
	}
	return ""
}

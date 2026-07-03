package lang

// reply.go — 根据 Issue/评论文本推断回复语言，供 Investigator、PR 描述等共用。

import (
	"fmt"
	"strings"
	"unicode"
)

// Kind 推断的主要自然语言。
type Kind int

const (
	Unknown Kind = iota
	Chinese
	English
)

// FromIssue 根据最新用户评论、Issue 正文与标题推断回复语言（优先最近评论）。
func FromIssue(title, body string, commentBodies []string) Kind {
	for i := len(commentBodies) - 1; i >= 0; i-- {
		if k := detectText(commentBodies[i]); k != Unknown {
			return k
		}
	}
	if k := detectText(body); k != Unknown {
		return k
	}
	return detectText(title)
}

func detectText(s string) Kind {
	s = strings.TrimSpace(s)
	if s == "" {
		return Unknown
	}
	var han, latinLetters int
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			han++
		} else if unicode.IsLetter(r) && r <= unicode.MaxASCII {
			latinLetters++
		}
	}
	if han >= 2 {
		return Chinese
	}
	if latinLetters >= 12 {
		return English
	}
	if han > 0 && han >= latinLetters/4 {
		return Chinese
	}
	if latinLetters >= 4 {
		return English
	}
	return Unknown
}

// ReplyDirective 供 LLM 系统/用户提示中强调回复语言。
func ReplyDirective(title, body string, commentBodies []string) string {
	switch FromIssue(title, body, commentBodies) {
	case Chinese:
		return "【语言】请用中文撰写回复，与 Issue 提问者语言一致；勿默认改用英文。"
	case English:
		return "【Language】Write the reply in English, matching the asker's language. Do not default to Chinese."
	default:
		return "【Language】Write the reply in the same language as the Issue title, body, and most recent user comments."
	}
}

// PRDescribeDirective PR 标题/正文语言要求。
func PRDescribeDirective(title, body string, commentBodies []string) string {
	switch FromIssue(title, body, commentBodies) {
	case Chinese:
		return "【语言】PR 标题与正文请使用中文（与关联 Issue 一致），技术术语可保留英文。"
	case English:
		return "【Language】Write the PR title and body in English (match the linked Issue). Keep technical identifiers as-is."
	default:
		return "【Language】Write the PR title and body in the same language as the linked Issue."
	}
}

// PROpenedComment 修库完成后回链 Issue 的评论（非 AI 模板句）。
func PROpenedComment(prURL, summary, title, body string, commentBodies []string) string {
	summary = strings.TrimSpace(summary)
	switch FromIssue(title, body, commentBodies) {
	case English:
		msg := fmt.Sprintf("Opened a PR to address this issue: %s", prURL)
		if summary != "" {
			msg += "\n\n" + summary
		}
		return msg
	default:
		msg := fmt.Sprintf("已开 PR 修复此 Issue：%s", prURL)
		if summary != "" {
			msg += "\n\n" + summary
		}
		return msg
	}
}

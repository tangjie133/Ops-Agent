package tui

// output_limit.go — 输出区与日志展示的字节/字符截断，防止 viewport 过大。

import "strings"

const maxOutputBytes = 48 * 1024
const maxLogDisplayChars = 320

func trimOutputContent(s string) string {
	if len(s) <= maxOutputBytes {
		return s
	}
	tail := s[len(s)-maxOutputBytes:]
	if i := strings.Index(tail, "\n"); i >= 0 {
		tail = tail[i+1:]
	}
	return "…（较早输出已截断）\n" + tail
}

func truncateForDisplay(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n\n…（已截断，完整内容已保存在草稿/报告中）"
}

func truncateLogDisplay(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLogDisplayChars {
		return text
	}
	return text[:maxLogDisplayChars] + "…"
}

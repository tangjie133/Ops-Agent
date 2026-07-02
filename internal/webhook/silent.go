package webhook

import (
	"net/http"
	"strings"
)

// silentEnqueueReason 常见、可预期的跳过原因：不向 TUI 发事件，避免刷屏拖慢界面。
func silentEnqueueReason(reason string) bool {
	switch reason {
	case "already active",
		"already queued or dismissed",
		"issue closed",
		"rule mismatch",
		"todo cap reached",
		"not in todo",
		"already inactive",
		"issue_watch disabled",
		"own auto reply",
		"refactor_pr comment approval disabled",
		"already confirmed":
		return true
	default:
		if strings.HasPrefix(reason, "status ") && strings.Contains(reason, " cannot confirm fix") {
			return true
		}
		return false
	}
}

func writeEnqueueSkip(w http.ResponseWriter, reason string) {
	writeJSON(w, map[string]any{"ok": true, "added": false, "reason": reason})
}

func writeRemoveSkip(w http.ResponseWriter, reason string) {
	writeJSON(w, map[string]any{"ok": true, "removed": false, "reason": reason})
}

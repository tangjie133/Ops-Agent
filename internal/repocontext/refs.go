package repocontext

import (
	"regexp"
	"strings"
)

var (
	reBacktickPath = regexp.MustCompile("`([^`\\n]+)`")
	reFileExt      = regexp.MustCompile(`(?i)(?:^|[\s('"[\{(])([\w./-]+\.(?:go|py|rs|js|ts|tsx|jsx|md|yaml|yml|toml|json|xml|html|css|scss|java|kt|swift|rb|sh|bash|c|cpp|h|hpp|cs|vue|sql|gradle|mod|sum))`)
	reAtFile       = regexp.MustCompile(`(?i)(?:^|\s)@([\w./-]+\.(?:go|py|rs|js|ts|tsx|md|yaml|yml|toml|json))`)
)

// ExtractFileRefs 从 Issue 文本中提取可能指向仓库文件的路径。
func ExtractFileRefs(text string) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		raw = strings.Trim(raw, ".,;:'\"")
		if raw == "" || strings.Contains(raw, "..") || strings.HasPrefix(raw, "http") {
			return
		}
		raw = normalizeRelPath(raw)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}

	for _, m := range reBacktickPath.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range reFileExt.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range reAtFile.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	return out
}

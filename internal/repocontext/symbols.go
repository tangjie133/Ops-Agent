package repocontext

import (
	"regexp"
	"strings"
)

var (
	reClassMethod = regexp.MustCompile(`\b([A-Za-z_][\w]*::[A-Za-z_]\w*)\b`)
	reConstIdent  = regexp.MustCompile(`\b([A-Z][A-Z0-9_]{2,})\b`)
	reEnumType    = regexp.MustCompile(`\b(e[A-Z][A-Za-z0-9_]*)\b`)
	reFuncCall    = regexp.MustCompile(`\b([A-Za-z_][\w]*)\s*\(`)
	reTypeName    = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]{2,})\b`)
)

var symbolStopWords = map[string]bool{
	"RTC": true, "INT": true, "SQW": true, "AM": true, "PM": true,
	"URL": true, "API": true, "I2C": true, "IIC": true, "USB": true,
	"THE": true, "AND": true, "FOR": true, "NOT": true, "YOU": true,
	"CAN": true, "ALL": true, "OUT": true, "PIN": true, "HZ": true,
}

// ExtractSearchTerms 从 Issue 文本提取应在仓库中搜索的符号（函数、寄存器、类名等）。
func ExtractSearchTerms(text string) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(term string, priority bool) {
		term = strings.TrimSpace(term)
		if len(term) < 3 {
			return
		}
		if symbolStopWords[strings.ToUpper(term)] || symbolStopWords[term] {
			return
		}
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		if priority {
			out = append([]string{term}, out...)
		} else {
			out = append(out, term)
		}
	}

	for _, m := range reClassMethod.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1], true)
			if idx := strings.Index(m[1], "::"); idx > 0 {
				add(m[1][:idx], true)
				add(m[1][idx+2:], true)
			}
		}
	}
	for _, m := range reConstIdent.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1], strings.Contains(m[1], "_REG_") || strings.HasPrefix(m[1], "SD3031_"))
		}
	}
	for _, m := range reEnumType.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1], true)
		}
	}
	for _, m := range reFuncCall.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1], false)
		}
	}
	for _, m := range reTypeName.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 && strings.Contains(m[1], "_") {
			add(m[1], true)
		}
	}

	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

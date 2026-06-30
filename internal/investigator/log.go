package investigator

import (
	"fmt"
	"strings"
)

// Logger 调试日志回调（TUI 日志区、文件等）。
type Logger func(line string)

func logf(log Logger, format string, args ...any) {
	if log == nil {
		return
	}
	log(fmt.Sprintf(format, args...))
}

func formatStep(ev StepEvent) string {
	ref := fmt.Sprintf("[%d/%d]", ev.Step, ev.MaxSteps)
	switch {
	case ev.Err != "":
		return fmt.Sprintf("Investigator %s %s ERROR: %s", ref, ev.Action, ev.Err)
	case ev.Action == ActionReply:
		return fmt.Sprintf("Investigator %s reply 完成", ref)
	case ev.Observation != "":
		return fmt.Sprintf("Investigator %s %s %s → %s", ref, ev.Action, ev.Detail, ev.Observation)
	default:
		return fmt.Sprintf("Investigator %s %s %s", ref, ev.Action, ev.Detail)
	}
}

func summarizeObservation(action, obs string) string {
	obs = strings.TrimSpace(obs)
	if obs == "" {
		return "(empty)"
	}
	if action == ActionWebSearch {
		n := strings.Count(obs, "\n   http")
		if n == 0 && obs == "no results" {
			return "0 条结果"
		}
		return fmt.Sprintf("%d 条结果", n+1)
	}
	if action == ActionRAGSearch {
		if obs == "no results" || strings.Contains(obs, "no results") {
			return "0 命中"
		}
		return fmt.Sprintf("%d 片段", strings.Count(obs, "--- hit"))
	}
	if action == ActionRepoValidate {
		if strings.Contains(obs, "FAIL") {
			return "FAIL"
		}
		return "PASS"
	}
	if action == ActionFetchURL {
		if strings.Contains(obs, "PDF 二进制") {
			return "PDF（未解析）"
		}
		if i := strings.Index(obs, "---\n"); i >= 0 {
			return fmt.Sprintf("%d 字符", len(obs)-i)
		}
	}
	if action == ActionSearch {
		if obs == "no matches" {
			return "0 命中"
		}
		return fmt.Sprintf("%d 行命中", strings.Count(obs, "\n")+1)
	}
	if len(obs) > 120 {
		return obs[:120] + "…"
	}
	return obs
}

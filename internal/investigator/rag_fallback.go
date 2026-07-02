package investigator

import (
	"context"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/rag"
)

const ragMissHint = `

── 本地知识库 RAG ──
未命中相关内容。若 rag_search 仍无结果，系统将自动执行 web_search。`

func isRAGNoResults(obs string) bool {
	obs = strings.TrimSpace(obs)
	return obs == "no results" || strings.Contains(obs, "no results (检查 knowledge")
}

func isWebSearchNoResults(obs string) bool {
	return strings.TrimSpace(obs) == "no results"
}

// webSearchQueryFromIssue 从 Issue 标题/正文构造联网搜索词。
func webSearchQueryFromIssue(title, allText string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		q := title
		lower := strings.ToLower(title)
		if !strings.Contains(lower, "datasheet") && !strings.Contains(lower, "manual") {
			q += " datasheet"
		}
		if len(q) > 120 {
			q = q[:120]
		}
		return q
	}
	q := rag.QueryFromText(allText)
	if len(q) > 120 {
		q = q[:120]
	}
	return q
}

func issueAllText(iss *github.Issue, comments []github.IssueComment) string {
	var b strings.Builder
	b.WriteString(iss.Title)
	b.WriteString("\n")
	b.WriteString(iss.Body)
	for _, c := range comments {
		b.WriteString("\n")
		b.WriteString(c.Body)
	}
	return b.String()
}

func (inv *Investigator) appendAutoWebSearchOnRAGMiss(ctx context.Context, tools *Toolbox, iss *github.Issue, comments []github.IssueComment, ragIdx *rag.Index, prompt string) string {
	if !inv.cfg.RAG.On() || ragIdx == nil {
		return prompt
	}
	if !inv.cfg.Investigator.WebSearchOn() {
		return prompt
	}
	allText := issueAllText(iss, comments)
	if rag.HasPromptHits(ragIdx, allText, inv.cfg.RAG.InjectTopK) {
		return prompt
	}
	query := webSearchQueryFromIssue(iss.Title, allText)
	webObs, ok := runAutoWebSearch(ctx, tools, inv.cfg.Investigator, inv.log, query)
	if !ok {
		logf(inv.log, "Investigator 知识库未命中，自动 web_search 无结果: %q", query)
		return prompt
	}
	logf(inv.log, "Investigator 知识库未命中，已自动 web_search: %q", query)
	return prompt + "\n\n── 自动 web_search（知识库未命中）──\n" + webObs
}

func maybeAppendAutoWebSearchAfterRAG(ctx context.Context, tools *Toolbox, cfg config.InvestigatorConfig, log Logger, ragQuery, obs string) string {
	if !isRAGNoResults(obs) {
		return obs
	}
	query := strings.TrimSpace(ragQuery)
	if query == "" {
		return obs
	}
	webObs, ok := runAutoWebSearch(ctx, tools, cfg, log, query)
	if !ok {
		logf(log, "Investigator rag_search 无命中，自动 web_search 无结果: %q", query)
		return obs
	}
	logf(log, "Investigator rag_search 无命中，已自动 web_search: %q", query)
	return obs + "\n\n── 自动 web_search（RAG 无命中）──\n" + webObs
}

func runAutoWebSearch(ctx context.Context, tools *Toolbox, cfg config.InvestigatorConfig, log Logger, query string) (string, bool) {
	if !cfg.WebSearchOn() {
		return "", false
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "", false
	}
	obs, err := tools.Run(ctx, Action{Action: ActionWebSearch, Query: query})
	if err != nil {
		logf(log, "Investigator 自动 web_search 失败: %v", err)
		return "", false
	}
	if isWebSearchNoResults(obs) {
		return "", false
	}
	return obs, true
}

package investigator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/rag"
	"github.com/ZzedJay/Ops-Agent/internal/repocontext"
	"github.com/ZzedJay/Ops-Agent/internal/repovalidate"
)

// Investigator 多轮 Agent Issue 分析器。
type Investigator struct {
	cfg       config.AIConfig
	proxy     config.ProxyConfig
	gh        *github.Client
	llm       LLM
	workspace *repocontext.Workspace
	observer  StepObserver
	log       Logger
}

func (inv *Investigator) SetLogger(log Logger) {
	inv.log = log
}

func New(cfg config.AIConfig, proxy config.ProxyConfig, gh *github.Client, llm LLM) *Investigator {
	cfg.Investigator.Normalize()
	cfg.RepoContext.Normalize()
	cfg.RAG.Normalize()
	proxy.Normalize()
	return &Investigator{
		cfg:       cfg,
		proxy:     proxy,
		gh:        gh,
		llm:       llm,
		workspace: repocontext.NewWorkspace(cfg.RepoContext, proxy, gh),
	}
}

func (inv *Investigator) SetObserver(obs StepObserver) {
	inv.observer = obs
}

// AnalyzeIssue 调查仓库并生成 GitHub 评论草稿。
func (inv *Investigator) AnalyzeIssue(ctx context.Context, repo string, num int) (string, error) {
	if inv.gh == nil {
		return "", fmt.Errorf("github client not configured")
	}
	if inv.llm == nil {
		return "", fmt.Errorf("llm not configured")
	}

	logf(inv.log, "Investigator 开始 %s#%d", repo, num)
	logf(inv.log, "Investigator 配置: web_search=%v web_fetch=%v rag=%v proxy=%v",
		inv.cfg.Investigator.WebSearchOn(), inv.cfg.Investigator.WebFetchOn(), inv.cfg.RAG.On(), inv.proxy.Summary())

	iss, err := inv.gh.IssueView(ctx, repo, num)
	if err != nil {
		logf(inv.log, "Investigator issue view 失败: %v", err)
		return "", err
	}
	if !strings.EqualFold(iss.State, "OPEN") {
		return "", fmt.Errorf("issue %s#%d is not open", repo, num)
	}
	logf(inv.log, "Investigator issue view OK · 评论 %d 条", len(iss.Comments))

	logf(inv.log, "Investigator 克隆/更新仓库 %s …", repo)
	repoPath, err := inv.workspace.Prepare(ctx, repo)
	if err != nil {
		logf(inv.log, "Investigator 仓库准备失败: %v", err)
		return "", fmt.Errorf("prepare repository: %w", err)
	}
	logf(inv.log, "Investigator 仓库就绪: %s", repoPath)

	var ragIdx *rag.Index
	if inv.cfg.RAG.On() && inv.cfg.RAG.ReindexOnAnalyzeOn() {
		idx, err := rag.EnsureKnowledgeIndex(ctx, inv.cfg.RAG, func(line string) {
			logf(inv.log, "%s", line)
		})
		if err != nil {
			logf(inv.log, "Investigator RAG 知识库索引失败: %v", err)
		} else {
			ragIdx = idx
		}
	}

	tools := NewToolbox(repoPath, inv.cfg.Investigator, inv.cfg.RAG, ragIdx, inv.proxy)
	tools.SetLogger(inv.log)
	loop := NewLoop(inv.cfg.Investigator, inv.llm, tools, inv.observer)
	loop.SetLogger(inv.log)

	prompt := buildIssuePrompt(repo, iss, ragIdx, inv.cfg.RAG, repoPath)
	if urls := ExtractHTTPURLs(iss.Title + "\n" + iss.Body); len(urls) > 0 {
		logf(inv.log, "Investigator Issue 链接 %d 个: %s", len(urls), strings.Join(urls, ", "))
	}

	return loop.Run(ctx, prompt)
}

func buildIssuePrompt(repo string, iss *github.Issue, ragIdx *rag.Index, ragCfg config.RAGConfig, repoPath string) string {
	var labels []string
	for _, l := range iss.Labels {
		labels = append(labels, l.Name)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "仓库: %s\nIssue: #%d\n标题: %s\n状态: %s\n标签: %s\n链接: %s\n\n正文:\n%s",
		repo, iss.Number, iss.Title, iss.State, strings.Join(labels, ", "), iss.URL, strings.TrimSpace(iss.Body))

	if len(iss.Comments) > 0 {
		b.WriteString("\n\n── Issue 评论 ──\n")
		for i, c := range iss.Comments {
			author := c.Author.Login
			if author == "" {
				author = "unknown"
			}
			fmt.Fprintf(&b, "\n[评论 %d — @%s]\n%s\n", i+1, author, strings.TrimSpace(c.Body))
		}
	}

	allText := iss.Title + "\n" + iss.Body
	for _, c := range iss.Comments {
		allText += "\n" + c.Body
	}
	if section := rag.PromptSection(ragIdx, allText, ragCfg.InjectTopK); section != "" {
		b.WriteString(section)
	}
	if ragCfg.DefaultStandard != "" && repoPath != "" {
		standardsDir := filepath.Join(config.KnowledgeDir(ragCfg), "standards")
		if std, err := repovalidate.LoadStandard(standardsDir, ragCfg.DefaultStandard); err == nil {
			report := repovalidate.Validate(repoPath, std)
			b.WriteString("\n\n── 仓库格式检测 (")
			b.WriteString(ragCfg.DefaultStandard)
			b.WriteString(") ──\n")
			b.WriteString(report.Format())
			b.WriteByte('\n')
		}
	}
	if urls := ExtractHTTPURLs(allText); len(urls) > 0 {
		b.WriteString("\n\n── Issue 中的链接（可用 fetch_url 读取）──\n")
		for _, u := range urls {
			b.WriteString(u + "\n")
		}
	}

	b.WriteString("\n\n请结合知识库（规范/数据手册）、repo_validate 与源码调查；完成后 reply。")
	return b.String()
}

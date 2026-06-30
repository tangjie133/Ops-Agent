package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
	"github.com/ZzedJay/Ops-Agent/internal/prcheck"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func runCommand(ctx context.Context, cfg *config.Config, gh *github.Client, store *todo.FileStore, line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if !strings.HasPrefix(line, "/") {
		return "" // handled by Agent in model
	}

	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help", "/h", "/?":
		return helpText()
	case "/status":
		return cmdStatus(ctx, gh, cfg)
	case "/mode":
		return "输入 /mode 后按 Enter 打开模式选择菜单。"
	case "/model", "/ai":
		return "输入 /model 后按 Enter 打开模型配置菜单。"
	case "/proxy", "/vpn", "/网络":
		return "输入 /proxy 后按 Enter 打开网络代理配置菜单。"
	case "/webhook":
		return "输入 /webhook 后按 Enter 打开 Webhook 配置菜单。"
	case "/check":
		return cmdCheck(ctx, gh)
	case "/issue":
		cwdRepo, _ := gh.RepoFromCwd(ctx)
		repo, num, errMsg := parseIssueArgs(parts, store, cwdRepo)
		if errMsg != "" {
			return errMsg
		}
		return cmdIssue(ctx, gh, store, repo, num)
	case "/feedback":
		return "反馈功能占位（M4）。"
	case "/clean", "/clear":
		return "" // handled in model: clears output before runCommand
	case "/logs":
		return fmt.Sprintf("复制全部日志: Ctrl+Y 或 /logs copy\n日志文件: %s", config.LogFilePath())
	default:
		return fmt.Sprintf("未知命令: %s\n输入 /help 查看可用命令。", cmd)
	}
}

func helpText() string {
	return `可用命令:
  /help              显示帮助
  /status            检查 gh 与 llama-server
  /clean             清空输出区域
  /logs              日志复制说明（Ctrl+Y 复制 · 文件路径）
  /webhook           打开 Webhook 配置菜单（二级菜单，自动保存）
  /model             打开模型配置菜单（base_url / model / api_key）
  /proxy             打开网络代理菜单（翻墙 / gh clone）
  /mode              打开模式选择菜单（1/2/3 或 j/k + Enter，自动保存）
  /check             检测当前分支 PR（checks + 冲突）
  /issue owner/repo#n  查看 issue 详情（i 键使用待办所属仓库）
  /feedback          反馈（M4）

` + config.FormatModesHelp("") + `

快捷键:
  Enter        发送（自然语言 → Agent）
  Tab / →      命令自动补全
  Ctrl+C       退出
  j / k        待办列表上/下（输入框为空时）
  i            查看选中待办 issue 详情
  p            发布 ready 草稿（semi 确认 / manual）
  d            忽略选中待办
  Ctrl+L       显示/隐藏日志区
  Ctrl+Y       复制全部日志到剪贴板
  鼠标滚轮     在对话/日志区滚动

说明: TUI 全屏模式下无法用鼠标拖选；请 Ctrl+Y 复制日志，或 tail 日志文件。

manual 模式聊天处理待办:
  j/k 选中 → 「分析/处理这条」生成草稿 → 「发布」发帖 → 「忽略」移除

semi/full: Worker 自动分析，ready 后按 p 发布`
}

func isOutputClearCommand(line string) bool {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "/clean", "/clear":
		return true
	default:
		return false
	}
}

func isLogsCopyCommand(line string) bool {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "/logs copy", "/logs cp":
		return true
	default:
		return false
	}
}

func cmdStatus(ctx context.Context, gh *github.Client, cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("── 环境状态 ──\n\n")

	if !gh.Available() {
		b.WriteString("GitHub CLI (gh): 未安装或不在 PATH\n")
	} else {
		auth, _ := gh.AuthStatus(ctx)
		if auth.LoggedIn {
			b.WriteString(fmt.Sprintf("GitHub CLI: 已登录 (%s @ %s)\n", auth.User, auth.Host))
		} else {
			b.WriteString("GitHub CLI: 未登录 — 请运行 gh auth login\n")
			if auth.Raw != "" {
				b.WriteString(auth.Raw + "\n")
			}
		}

		repo, err := gh.RepoFromCwd(ctx)
		if err != nil {
			b.WriteString(fmt.Sprintf("当前仓库: 无法解析（%v）\n", err))
		} else {
			b.WriteString(fmt.Sprintf("当前仓库: %s\n", repo))
		}
	}

	health := ai.CheckHealth(ctx, cfg.AI)
	if health.Reachable {
		b.WriteString(fmt.Sprintf("llama-server: %s\n", health.Message))
	} else {
		b.WriteString(fmt.Sprintf("llama-server: %s\n", health.Message))
	}

	b.WriteString(fmt.Sprintf("网络代理: %s\n", cfg.Proxy.Summary()))
	ghHealth := netproxy.CheckGitHubWithGH(ctx, cfg.Proxy)
	if ghHealth.Reachable {
		b.WriteString("GitHub 访问: " + ghHealth.Message + "\n")
	} else {
		b.WriteString("GitHub 访问: " + ghHealth.Message + "\n")
	}

	b.WriteString(fmt.Sprintf("\n自动化模式: %s\n", cfg.IssueAutomation.ModeSummary()))
	b.WriteString(fmt.Sprintf("Issue 监视: enabled=%v labels=%v (webhook)\n",
		cfg.IssueWatch.Enabled, cfg.IssueWatch.Labels))
	if cfg.Webhook.Enabled {
		b.WriteString(fmt.Sprintf("Webhook: %s (%s)\n", cfg.Webhook.LocalURL(), cfg.WebhookSummary()))
	} else {
		b.WriteString("Webhook: 已禁用\n")
	}
	b.WriteString(fmt.Sprintf("待办存储: %s\n", config.TodoStorePath()))

	return b.String()
}

func cmdCheck(ctx context.Context, gh *github.Client) string {
	res, err := prcheck.Check(ctx, gh, prcheck.Options{})
	if err != nil {
		return fmt.Sprintf("PR 检测失败: %v", err)
	}
	return res.FormatReport()
}

func cmdIssue(ctx context.Context, gh *github.Client, store *todo.FileStore, repo string, num int) string {
	iss, err := gh.IssueView(ctx, repo, num)
	if err != nil {
		return fmt.Sprintf("读取 issue 失败 (%s): %v", formatIssueRef(repo, num), err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("── %s ──\n\n", formatIssueRef(repo, iss.Number)))
	b.WriteString(iss.Title + "\n")
	b.WriteString(fmt.Sprintf("状态: %s\n", iss.State))
	if iss.URL != "" {
		b.WriteString(iss.URL + "\n")
	}
	if len(iss.Labels) > 0 {
		names := make([]string, len(iss.Labels))
		for i, l := range iss.Labels {
			names[i] = l.Name
		}
		b.WriteString("标签: " + strings.Join(names, ", ") + "\n")
	}
	if len(iss.Assignees) > 0 {
		names := make([]string, len(iss.Assignees))
		for i, u := range iss.Assignees {
			names[i] = u.Login
		}
		b.WriteString("指派: " + strings.Join(names, ", ") + "\n")
	} else {
		b.WriteString("指派: (未指派)\n")
	}

	if item, ok := store.Get(repo, num); ok {
		b.WriteString(fmt.Sprintf("\n待办状态: %s\n", item.Status))
		if item.Draft != "" {
			b.WriteString("\n草稿:\n" + item.Draft + "\n")
		}
	} else {
		b.WriteString("\n待办: 未收录\n")
	}
	return b.String()
}

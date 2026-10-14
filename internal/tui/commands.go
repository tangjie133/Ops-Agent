package tui

// commands.go — 斜杠命令（/help、/status 等）的执行逻辑。

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func runCommand(ctx context.Context, cfg *config.Config, gh *github.Client, store *task.FileStore, line string) string {
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
	case "/mode", "/model", "/ai", "/proxy", "/vpn", "/网络", "/webhook":
		return "这些配置不在菜单里。空输入按 s 打开设置页，或在对话里说「模型 …」「webhook 开」「代理 …」。"
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
		return fmt.Sprintf("会话日志: %s\n诊断日志: %s\n复制日志: Ctrl+Y\n查看: tail -f %s",
			config.LogFilePath(), config.DiagLogFilePath(), config.LogFilePath())
	default:
		return fmt.Sprintf("未知命令: %s\n输入 /help 查看可用命令。", cmd)
	}
}

func helpText() string {
	return `可用命令:
  /help              显示帮助
  /status            检查 gh 与 llama-server
  /clean             清空输出区域
  /logs              日志文件路径（tail -f 或 Ctrl+Y 复制）
  /issue owner/repo#n  查看 issue 详情

快捷键:
  输入框为空时 j / k   选择任务
  输入框为空时 s       打开设置页（模型、webhook、代理）
  Esc                  主界面退出；设置页返回
  Enter                发送对话或斜杠命令
  Ctrl+Y               复制 tui.log 到剪贴板

说明: 新 Issue 进入任务队列，由编排器修改代码、测试并创建 draft PR。模型、webhook、代理在设置页或对话里改。合并、关闭 Issue、force push 需要下一句输入「确认」。`
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
	b.WriteString(fmt.Sprintf("修库 PR: %s\n", cfg.IssueAutomation.RefactorPR.Summary()))
	b.WriteString(fmt.Sprintf("Issue 监视: enabled=%v labels=%v (webhook)\n",
		cfg.IssueWatch.Enabled, cfg.IssueWatch.Labels))
	if cfg.Webhook.Enabled {
		b.WriteString(fmt.Sprintf("Webhook: %s (%s)\n", cfg.Webhook.LocalURL(), cfg.WebhookSummary()))
	} else {
		b.WriteString("Webhook: 已禁用\n")
	}
	b.WriteString(fmt.Sprintf("任务存储: %s\n", config.TaskStorePath(cfg)))

	return b.String()
}

func cmdIssue(ctx context.Context, gh *github.Client, store *task.FileStore, repo string, num int) string {
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

	if item, ok := store.FindIssue(repo, num); ok {
		b.WriteString(fmt.Sprintf("\n任务状态: %s\n", item.Status))
		if item.PRURL != "" {
			b.WriteString("PR: " + item.PRURL + "\n")
		}
	} else {
		b.WriteString("\n任务: 未收录\n")
	}
	return b.String()
}

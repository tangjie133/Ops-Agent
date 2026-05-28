package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
	"github.com/ZzedJay/Ops-Agent/internal/prcheck"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func runCommand(ctx context.Context, cfg *config.Config, gh *github.Client, store *todo.FileStore, line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if !strings.HasPrefix(line, "/") {
		return fmt.Sprintf("收到: %s\n\n（Agent 将在 M3 实现；可先使用 /status、/mode、/help）", line)
	}

	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help", "/h", "/?":
		return helpText()
	case "/status":
		return cmdStatus(ctx, gh, cfg)
	case "/mode":
		if len(parts) < 2 {
			return fmt.Sprintf("当前模式: %s\n用法: /mode manual|semi|full", cfg.IssueAutomation.ModeLabel())
		}
		cfg.IssueAutomation.SetMode(parts[1])
		return fmt.Sprintf("已切换为: %s", cfg.IssueAutomation.ModeLabel())
	case "/check":
		return cmdCheck(ctx, gh)
	case "/issue":
		if len(parts) < 2 {
			return "用法: /issue <number>"
		}
		var num int
		if _, err := fmt.Sscanf(parts[1], "%d", &num); err != nil || num <= 0 {
			return "无效的 issue 编号"
		}
		return cmdIssue(ctx, gh, store, num)
	case "/feedback":
		return "反馈功能占位（M4）。"
	case "/clean", "/clear":
		return "" // handled in model: clears output before runCommand
	default:
		return fmt.Sprintf("未知命令: %s\n输入 /help 查看可用命令。", cmd)
	}
}

func helpText() string {
	return `可用命令:
  /help              显示帮助
  /status            检查 gh 与 llama-server
  /clean             清空输出区域
  /mode [manual|semi|full]  切换 Issue 自动化模式
  /check             检测当前分支 PR（checks + 冲突）
  /issue <n>         查看 issue 详情与待办状态
  /feedback          反馈（M4）

快捷键:
  Enter        发送
  Tab / →      命令自动补全
  Ctrl+C       退出
  M            切换 manual → semi → full
  j / k          待办列表上/下（输入框为空时）
  i              查看选中待办 issue 详情
  d              忽略选中待办
  鼠标滚轮       在中间输出区滚动历史`
}

func isOutputClearCommand(line string) bool {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "/clean", "/clear":
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

	b.WriteString(fmt.Sprintf("\n自动化模式: %s\n", cfg.IssueAutomation.ModeLabel()))
	b.WriteString(fmt.Sprintf("Issue 监视: enabled=%v labels=%v (webhook)\n",
		cfg.IssueWatch.Enabled, cfg.IssueWatch.Labels))
	if cfg.Webhook.Enabled {
		b.WriteString(fmt.Sprintf("Webhook 本地: %s\n", cfg.Webhook.LocalURL()))
		if cfg.Webhook.PublicURL != "" {
			b.WriteString(fmt.Sprintf("GitHub App URL: %s\n", cfg.Webhook.PublicURL))
		}
		b.WriteString(fmt.Sprintf("Secret 已配置: %v\n", cfg.Webhook.Secret != ""))
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

func cmdIssue(ctx context.Context, gh *github.Client, store *todo.FileStore, num int) string {
	repo, err := gh.RepoFromCwd(ctx)
	if err != nil {
		return fmt.Sprintf("无法解析仓库: %v", err)
	}
	iss, err := gh.IssueView(ctx, repo, num)
	if err != nil {
		return fmt.Sprintf("读取 issue 失败: %v", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("── Issue #%d ──\n\n", iss.Number))
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

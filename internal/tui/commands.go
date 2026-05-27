package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/github"
)

func runCommand(ctx context.Context, cfg *config.Config, gh *github.Client, line string) string {
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
		return "PR 检测将在 M2 实现。请先使用 /status 检查环境。"
	case "/issue":
		if len(parts) < 2 {
			return "用法: /issue <number>"
		}
		return fmt.Sprintf("聚焦 issue #%s（详情视图 M2.5）", parts[1])
	case "/feedback":
		return "反馈功能占位（M4）。"
	default:
		return fmt.Sprintf("未知命令: %s\n输入 /help 查看可用命令。", cmd)
	}
}

func helpText() string {
	return `可用命令:
  /help              显示帮助
  /status            检查 gh 与 llama-server
  /mode [manual|semi|full]  切换 Issue 自动化模式
  /check             PR 检测（M2）
  /issue <n>         聚焦 issue（M2.5）
  /feedback          反馈（M4）

快捷键:
  Enter   发送
  Ctrl+C  退出
  M       切换 manual → semi → full`
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
	b.WriteString(fmt.Sprintf("Issue 监视: enabled=%v labels=%v\n",
		cfg.IssueWatch.Enabled, cfg.IssueWatch.Labels))

	return b.String()
}

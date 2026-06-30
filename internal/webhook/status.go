package webhook

import (
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// FormatStatus 返回 webhook 接入状态与配置说明。
func FormatStatus(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("── Webhook 接入 ──\n\n")

	if !cfg.Webhook.Enabled {
		b.WriteString("状态: 已禁用\n")
		b.WriteString("在配置中设置 webhook.enabled: true 后重启。\n")
		return b.String()
	}

	b.WriteString("状态: 已启用\n")
	b.WriteString(config.WebhookConnectionIntro() + "\n\n")
	b.WriteString(fmt.Sprintf("本地 URL: %s\n", cfg.Webhook.LocalURL()))
	b.WriteString(fmt.Sprintf("健康检查: %s\n", cfg.Webhook.HealthURL()))

	fields := config.WebhookConnFields()
	labels := []string{"listen", "path", "secret", "public_url"}
	for i, f := range fields {
		var val string
		switch i {
		case 0:
			val = cfg.Webhook.Listen
		case 1:
			val = cfg.Webhook.Path
		case 2:
			val = config.FormatWebhookSecretDisplay(cfg.Webhook.Secret)
		case 3:
			val = config.FormatWebhookPublicURLDisplay(cfg.Webhook.PublicURL)
		}
		b.WriteString(fmt.Sprintf("\n%s (%s)\n  %s\n  当前: %s\n", f.Title, labels[i], f.Description, val))
	}

	b.WriteString("\n── Issue 入队规则 ──\n")
	b.WriteString(fmt.Sprintf("issue_watch.enabled: %v\n", cfg.IssueWatch.Enabled))
	b.WriteString(fmt.Sprintf("labels (OR): %v\n", cfg.IssueWatch.Labels))
	b.WriteString(fmt.Sprintf("require_unassigned: %v\n", cfg.IssueWatch.RequireUnassigned))
	b.WriteString(fmt.Sprintf("todo 上限: %d\n", cfg.IssueWatch.Todo.MaxItems))

	b.WriteString(fmt.Sprintf("Smee 隧道: %s\n", cfg.Webhook.SmeeStatusLabel()))

	b.WriteString("\n── 接入步骤（Smee）──\n")
	b.WriteString("1. 打开 https://smee.io 生成频道 URL\n")
	b.WriteString("2. /webhook → 连接配置 → Public URL 填 smee 频道\n")
	b.WriteString("3. /webhook → Smee 隧道 → 已启用（默认开启）\n")
	b.WriteString("4. GitHub Webhooks → Payload URL = Public URL，勾选 Issues 与 Issue comments\n")
	b.WriteString("5. 启动 ./ops-agent，无需再单独运行 smee-client\n")

	b.WriteString("\n本地测试: make webhook-test（需 ops-agent 已运行）\n")
	return b.String()
}

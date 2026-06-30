package netproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// GitHubHealth GitHub 连通性检测结果。
type GitHubHealth struct {
	Reachable bool
	Message   string
}

// CheckGitHub 检测 GitHub 是否可达（可选经代理）。
func CheckGitHub(ctx context.Context, proxy config.ProxyConfig) GitHubHealth {
	proxy.Normalize()
	if !proxy.Enabled || proxy.EffectiveHTTPS() == "" {
		return checkGitHubDirect(ctx)
	}
	client := HTTPClient(proxy, 12*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://github.com", nil)
	if err != nil {
		return GitHubHealth{Message: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return GitHubHealth{Message: "经代理连接失败: " + err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 && resp.StatusCode != 405 {
		return GitHubHealth{Message: fmt.Sprintf("HTTP %s", resp.Status)}
	}
	return GitHubHealth{
		Reachable: true,
		Message:   "GitHub 可达（经代理 " + config.FormatProxyDisplay(proxy.EffectiveHTTPS()) + "）",
	}
}

func checkGitHubDirect(ctx context.Context) GitHubHealth {
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://github.com", nil)
	if err != nil {
		return GitHubHealth{Message: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return GitHubHealth{Message: "直连失败: " + err.Error() + "（可 /proxy 启用代理）"}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return GitHubHealth{Reachable: true, Message: "GitHub 直连可达"}
}

// CheckGitHubWithGH 额外用 gh api 验证（需 gh 已安装）。
func CheckGitHubWithGH(ctx context.Context, proxy config.ProxyConfig) GitHubHealth {
	if _, err := exec.LookPath("gh"); err != nil {
		return CheckGitHub(ctx, proxy)
	}
	cmd := exec.CommandContext(ctx, "gh", "api", "rate_limit", "-q", ".resources.core.remaining")
	ConfigureCmd(cmd, proxy)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return GitHubHealth{Message: "gh api 失败: " + msg}
	}
	return GitHubHealth{Reachable: true, Message: "gh 经当前代理可访问 GitHub"}
}

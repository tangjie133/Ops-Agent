package netproxy

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// ConfigureCmd 为子进程注入代理环境变量（不修改全局 shell）。
func ConfigureCmd(cmd *exec.Cmd, proxy config.ProxyConfig) {
	cmd.Env = Environ(os.Environ(), proxy)
}

// Environ 在 base 环境上叠加或覆盖 HTTP(S)_PROXY。
func Environ(base []string, proxy config.ProxyConfig) []string {
	proxy.Normalize()
	if !proxy.Enabled {
		return base
	}
	out := stripProxyKeys(base)
	if hp := proxy.HTTPProxy; hp != "" {
		out = append(out, "HTTP_PROXY="+hp, "http_proxy="+hp)
	}
	if hsp := proxy.EffectiveHTTPS(); hsp != "" {
		out = append(out, "HTTPS_PROXY="+hsp, "https_proxy="+hsp)
	}
	if np := proxy.NoProxy; np != "" {
		out = append(out, "NO_PROXY="+np, "no_proxy="+np)
	}
	return out
}

func stripProxyKeys(env []string) []string {
	var out []string
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY":
			continue
		}
		out = append(out, e)
	}
	return out
}

// HTTPClient 返回带代理的 http.Client（用于连通性检测）。
func HTTPClient(proxy config.ProxyConfig, timeout time.Duration) *http.Client {
	proxy.Normalize()
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if proxy.Enabled {
		if ep := proxy.EffectiveHTTPS(); ep != "" {
			if u, err := url.Parse(ep); err == nil {
				tr.Proxy = http.ProxyURL(u)
			}
		}
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

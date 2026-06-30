package investigator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

var htmlTagRE = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]+>`)

type urlFetcher struct {
	cfg   config.InvestigatorConfig
	proxy config.ProxyConfig
}

func (t *Toolbox) fetchURL(ctx context.Context, rawURL string) (string, error) {
	if !t.cfg.WebFetchOn() {
		return "", fmt.Errorf("web fetch disabled in config")
	}
	u, err := validateFetchURL(rawURL)
	if err != nil {
		return "", err
	}

	logf(t.log, "Investigator fetch_url: %s (proxy=%v)", u.String(), t.proxy.Summary())

	timeout := time.Duration(t.cfg.FetchTimeoutSec) * time.Second
	client := netproxy.HTTPClient(t.proxy, timeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Ops-Agent/1.0 (Issue investigator)")
	req.Header.Set("Accept", "text/html,application/pdf,text/plain,application/json,*/*")

	resp, err := client.Do(req)
	if err != nil {
		logf(t.log, "Investigator fetch_url 失败: %v", err)
		return "", fmt.Errorf("HTTP 请求失败 (需 /proxy?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		logf(t.log, "Investigator fetch_url HTTP %s", resp.Status)
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}

	max := t.cfg.FetchMaxBytes
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(max+1)))
	if err != nil {
		return "", err
	}
	truncated := len(body) > max
	if truncated {
		body = body[:max]
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	logf(t.log, "Investigator fetch_url OK · %s · %d bytes", ct, len(body))

	var text string
	switch {
	case strings.Contains(ct, "pdf"):
		return fmt.Sprintf("url: %s\ncontent-type: %s\n(PDF 二进制，当前无法解析正文；请 web_search 找 HTML 版 datasheet 或厂商 wiki)", u.String(), ct), nil
	case strings.Contains(ct, "html"):
		text = htmlToText(string(body))
	default:
		text = string(body)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "url: %s\ncontent-type: %s\n", u.String(), ct)
	if truncated {
		b.WriteString("(content truncated)\n")
	}
	b.WriteString("---\n")
	b.WriteString(strings.TrimSpace(text))
	return b.String(), nil
}

func htmlToText(html string) string {
	html = htmlTagRE.ReplaceAllString(html, " ")
	html = regexp.MustCompile(`\s+`).ReplaceAllString(html, " ")
	return strings.TrimSpace(html)
}

func validateFetchURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("only http/https allowed")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if isBlockedHost(host) {
		return nil, fmt.Errorf("blocked host")
	}
	return u, nil
}

func isBlockedHost(host string) bool {
	blocked := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	for _, b := range blocked {
		if host == b {
			return true
		}
	}
	if strings.HasPrefix(host, "127.") {
		return true
	}
	if strings.HasPrefix(host, "10.") {
		return true
	}
	if strings.HasPrefix(host, "192.168.") {
		return true
	}
	if strings.HasPrefix(host, "169.254.") {
		return true
	}
	if host == "metadata.google.internal" {
		return true
	}
	return false
}

var httpURLRE = regexp.MustCompile(`https?://[^\s\])>"']+`)

// ExtractHTTPURLs 从文本提取 http(s) 链接。
func ExtractHTTPURLs(text string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range httpURLRE.FindAllString(text, -1) {
		m = strings.TrimRight(m, ".,;)")
		if _, ok := seen[m]; ok {
			continue
		}
		if _, err := validateFetchURL(m); err != nil {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
		if len(out) >= 10 {
			break
		}
	}
	return out
}

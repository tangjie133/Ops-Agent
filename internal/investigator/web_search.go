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

	"github.com/ZzedJay/Ops-Agent/internal/netproxy"
)

var (
	ddgResultLinkRE = regexp.MustCompile(`<a rel="nofollow" class="result__a" href="([^"]+)"[^>]*>([^<]*)</a>`)
	ddgSnippetRE    = regexp.MustCompile(`<a class="result__snippet"[^>]*>([^<]*)</a>`)
)

type searchHit struct {
	Title   string
	URL     string
	Snippet string
}

func (t *Toolbox) webSearch(ctx context.Context, query string) (string, error) {
	if !t.cfg.WebSearchOn() {
		return "", fmt.Errorf("web search disabled in config")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("empty query")
	}

	logf(t.log, "Investigator web_search 请求: %q (proxy=%v)", query, t.proxy.Summary())
	hits, meta, err := t.searchDuckDuckGo(ctx, query)
	if err != nil {
		logf(t.log, "Investigator web_search 错误: %v", err)
		return "", err
	}
	logf(t.log, "Investigator web_search DDG: %s", meta)
	if len(hits) == 0 {
		logf(t.log, "Investigator web_search 无结果 — 检查 /proxy 或 DuckDuckGo 是否可达")
		return "no results", nil
	}

	var b strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, h.Title, h.URL)
		if h.Snippet != "" {
			fmt.Fprintf(&b, "   %s\n", h.Snippet)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func (t *Toolbox) searchDuckDuckGo(ctx context.Context, query string) ([]searchHit, string, error) {
	form := url.Values{}
	form.Set("q", query)

	timeout := time.Duration(t.cfg.FetchTimeoutSec) * time.Second
	client := netproxy.HTTPClient(t.proxy, timeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Ops-Agent/1.0 (Issue investigator)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP 请求失败 (需 /proxy?): %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512_000))
	if err != nil {
		return nil, "", err
	}
	meta := fmt.Sprintf("HTTP %s · body %d bytes", resp.Status, len(body))
	if resp.StatusCode >= 400 {
		return nil, meta, fmt.Errorf("search HTTP %s", resp.Status)
	}

	hits := parseDDGHTML(string(body), t.cfg.WebSearchMaxResults)
	if len(hits) == 0 {
		meta += fmt.Sprintf(" · parsed 0 hits (HTML 结构变化或被拦截?)")
	}
	return hits, meta, nil
}

func parseDDGHTML(html string, max int) []searchHit {
	if max <= 0 {
		max = 8
	}
	links := ddgResultLinkRE.FindAllStringSubmatch(html, max)
	snippets := ddgSnippetRE.FindAllStringSubmatch(html, max)

	var out []searchHit
	for i, m := range links {
		if len(m) < 3 {
			continue
		}
		hit := searchHit{
			URL:   decodeDDGRedirect(m[1]),
			Title: strings.TrimSpace(htmlUnescape(m[2])),
		}
		if i < len(snippets) && len(snippets[i]) > 1 {
			hit.Snippet = strings.TrimSpace(htmlUnescape(snippets[i][1]))
		}
		if hit.URL == "" {
			continue
		}
		out = append(out, hit)
	}
	return out
}

func decodeDDGRedirect(href string) string {
	href = strings.TrimSpace(href)
	if !strings.Contains(href, "uddg=") {
		return href
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	if v := u.Query().Get("uddg"); v != "" {
		return v
	}
	return href
}

func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

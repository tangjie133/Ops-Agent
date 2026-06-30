package smee

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Client 订阅 smee 频道并将 webhook 转发到本地 target URL。
type Client struct {
	channelURL string
	targetURL  string
	streamClient  *http.Client
	forwardClient *http.Client
	logger     *log.Logger

	cancel context.CancelFunc
}

func NewClient(channelURL, targetURL string, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.Default()
	}
	return &Client{
		channelURL: strings.TrimSpace(channelURL),
		targetURL:  strings.TrimSpace(targetURL),
		// SSE 长连接不能用整体 Timeout，否则约 30s 就断线重连。
		streamClient:  &http.Client{},
		forwardClient: &http.Client{Timeout: 30 * time.Second},
		logger:        logger,
	}
}

func (c *Client) Start(ctx context.Context) {
	if c.channelURL == "" || c.targetURL == "" {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go c.run(runCtx)
}

func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (c *Client) run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.streamOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		c.logger.Printf("smee · 断开: %v（%s 后重连）", err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) streamOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.channelURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("smee connect %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

		c.logger.Printf("smee · 已连接 %s → %s", c.channelURL, c.targetURL)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	var eventName string
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		name := eventName
		raw := strings.Join(dataLines, "\n")
		eventName = ""
		dataLines = nil

		if name != "" && name != "message" {
			return nil
		}
		if err := c.forward(raw); err != nil {
			c.logger.Printf("smee · 转发失败: %v", err)
		}
		return nil
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(line[len("event:"):])
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(line[len("data:"):]))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("stream ended")
}

func (c *Client) forward(raw string) error {
	headers, body, err := parseSmeeEvent(raw)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}

	req, err := http.NewRequest(http.MethodPost, c.targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.forwardClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("local webhook %s", resp.Status)
	}
	return nil
}

// parseSmeeEvent 解析 smee.io SSE data 行。
// smee.io 将 HTTP 头平铺在 JSON 顶层，body 为 JSON 对象；亦兼容旧版 headers 嵌套格式。
func parseSmeeEvent(raw string) (http.Header, []byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, nil, fmt.Errorf("parse payload: %w", err)
	}

	bodyRaw, hasBody := fields["body"]
	if !hasBody {
		return nil, nil, nil
	}
	body, err := decodeSmeeBody(bodyRaw)
	if err != nil {
		return nil, nil, err
	}
	if len(body) == 0 {
		return nil, nil, nil
	}

	headers := make(http.Header)
	if hdrRaw, ok := fields["headers"]; ok {
		var nested map[string]string
		if err := json.Unmarshal(hdrRaw, &nested); err == nil {
			for k, v := range nested {
				if hopByHopHeader(strings.ToLower(k)) {
					continue
				}
				headers.Set(k, v)
			}
			return headers, body, nil
		}
	}

	for k, v := range fields {
		if smeeMetaField(k) || smeeInfraField(k) || strings.EqualFold(k, "headers") {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			continue
		}
		if hopByHopHeader(strings.ToLower(k)) {
			continue
		}
		headers.Set(k, s)
	}
	return headers, body, nil
}

func decodeSmeeBody(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []byte(s), nil
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("invalid body json")
	}
	return append([]byte(nil), raw...), nil
}

func smeeMetaField(name string) bool {
	switch strings.ToLower(name) {
	case "body", "query", "timestamp":
		return true
	default:
		return false
	}
}

func smeeInfraField(name string) bool {
	lk := strings.ToLower(name)
	switch {
	case lk == "host", lk == "client-ip", lk == "max-forwards", lk == "disguised-host", lk == "was-default-hostname":
		return true
	case strings.HasPrefix(lk, "x-arr-"), strings.HasPrefix(lk, "x-forwarded-"), strings.HasPrefix(lk, "x-original-"),
		strings.HasPrefix(lk, "x-waws-"), strings.HasPrefix(lk, "x-appservice-"), strings.HasPrefix(lk, "x-site-deployment-"),
		strings.HasPrefix(lk, "x-client-"):
		return true
	default:
		return false
	}
}

func hopByHopHeader(name string) bool {
	switch name {
	case "host", "content-length", "connection", "keep-alive", "transfer-encoding", "te", "trailer", "upgrade", "proxy-authorization", "proxy-authenticate":
		return true
	default:
		return false
	}
}

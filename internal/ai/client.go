package ai

// client.go — OpenAI 兼容 chat/completions HTTP 客户端。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// Client 调用配置的 BaseURL + Model 进行对话补全。
type Client struct {
	cfg    config.AIConfig
	http   *http.Client
	apiURL string
}

// NewClient 根据 AIConfig 构造 HTTP 客户端。单次请求不设固定超时，由调用方的 ctx 决定等待多久。
func NewClient(cfg config.AIConfig) *Client {
	return &Client{
		cfg:    cfg,
		http:   &http.Client{},
		apiURL: chatCompletionsURL(cfg.BaseURL),
	}
}

func chatCompletionsURL(base string) string {
	base = strings.TrimSpace(base)
	base = strings.TrimSuffix(base, "/")
	if base == "" {
		return ""
	}
	return base + "/chat/completions"
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role             string          `json:"role"`
	Content          json.RawMessage `json:"content"`
	ReasoningContent string          `json:"reasoning_content"`
	Reasoning        string          `json:"reasoning"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
		Text    string      `json:"text"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Message 多轮对话消息。
type Message struct {
	Role    string
	Content string
}

// Chat 调用 OpenAI 兼容 chat/completions API。
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	msgs := []Message{}
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, Message{Role: "system", Content: system})
	}
	msgs = append(msgs, Message{Role: "user", Content: user})
	return c.ChatMessages(ctx, msgs)
}

// ChatMessages 多轮 messages 调用 chat/completions。
func (c *Client) ChatMessages(ctx context.Context, msgs []Message) (string, error) {
	if c.apiURL == "" {
		return "", fmt.Errorf("ai.base_url not configured")
	}
	if strings.TrimSpace(c.cfg.Model) == "" {
		return "", fmt.Errorf("ai.model not configured")
	}
	if len(msgs) == 0 {
		return "", fmt.Errorf("empty messages")
	}

	apiMsgs := make([]chatMessage, len(msgs))
	for i, m := range msgs {
		raw, _ := json.Marshal(m.Content)
		apiMsgs[i] = chatMessage{Role: m.Role, Content: raw}
	}

	body, err := json.Marshal(chatRequest{
		Model:       c.cfg.Model,
		Messages:    apiMsgs,
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" && c.cfg.APIKey != "local" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", err
	}

	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("parse ai response: %w", err)
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("ai error: %s", out.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ai http %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("ai returned empty content")
	}
	text := choiceText(out.Choices[0].Message, out.Choices[0].Text)
	if text == "" {
		return "", fmt.Errorf("ai returned empty content")
	}
	return text, nil
}

func choiceText(msg chatMessage, fallback string) string {
	if s := strings.TrimSpace(decodeMessageContent(msg.Content)); s != "" {
		return s
	}
	if s := strings.TrimSpace(msg.ReasoningContent); s != "" {
		return s
	}
	if s := strings.TrimSpace(msg.Reasoning); s != "" {
		return s
	}
	return strings.TrimSpace(fallback)
}

func decodeMessageContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return strings.Trim(string(raw), `"`)
}

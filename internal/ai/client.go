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
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const defaultTimeout = 120 * time.Second

// Client 调用配置的 BaseURL + Model 进行对话补全。
type Client struct {
	cfg    config.AIConfig
	http   *http.Client
	apiURL string
}

// NewClient 根据 AIConfig 构造 HTTP 客户端。
func NewClient(cfg config.AIConfig) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: defaultTimeout,
		},
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
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
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
		apiMsgs[i] = chatMessage{Role: m.Role, Content: m.Content}
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

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
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
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("ai returned empty content")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

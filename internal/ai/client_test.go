package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestChatCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello reply"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(config.AIConfig{
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
		APIKey:  "local",
	})
	out, err := c.Chat(context.Background(), "sys", "user msg")
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello reply" {
		t.Fatalf("got %q", out)
	}
}

func TestChatClientHasNoHardTimeout(t *testing.T) {
	c := NewClient(config.AIConfig{BaseURL: "http://127.0.0.1:1/v1", Model: "m"})
	if c.http.Timeout != 0 {
		t.Fatalf("http timeout=%s; chat must follow the caller's context only", c.http.Timeout)
	}
}

func TestChatCanceledContext(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	c := NewClient(config.AIConfig{BaseURL: srv.URL + "/v1", Model: "m", APIKey: "local"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Chat(ctx, "s", "u")
	if err == nil {
		t.Fatal("expected canceled context to fail")
	}
}

func TestChatUsesReasoningContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"{\"action\":\"reply\",\"body\":\"from think\"}"}}]}`))
	}))
	defer srv.Close()
	c := NewClient(config.AIConfig{BaseURL: srv.URL + "/v1", Model: "m", APIKey: "local"})
	out, err := c.Chat(context.Background(), "", "u")
	if err != nil || out != `{"action":"reply","body":"from think"}` {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestChatEmptyBaseURL(t *testing.T) {
	c := NewClient(config.AIConfig{})
	_, err := c.Chat(context.Background(), "s", "u")
	if err == nil {
		t.Fatal("expected error")
	}
}

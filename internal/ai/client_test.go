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

func TestChatEmptyBaseURL(t *testing.T) {
	c := NewClient(config.AIConfig{})
	_, err := c.Chat(context.Background(), "s", "u")
	if err == nil {
		t.Fatal("expected error")
	}
}

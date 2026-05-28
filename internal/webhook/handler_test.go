package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestIssueOpenedEnqueue(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "test-secret"
	cfg.IssueWatch.Labels = nil
	cfg.IssueWatch.RequireUnassigned = true

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	var added todo.Item
	h := NewHandler(cfg, store, func(item todo.Item) { added = item })

	body := []byte(`{
		"action":"opened",
		"issue":{"number":42,"title":"hello","state":"open","html_url":"https://github.com/o/r/issues/42","labels":[],"assignees":[]},
		"repository":{"full_name":"o/r"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", signBody("test-secret", body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if added.Number != 42 {
		t.Fatalf("callback not fired: %+v", added)
	}
	got, ok := store.Get("o/r", 42)
	if !ok || got.Status != todo.StatusInTodo {
		t.Fatalf("store item: %+v ok=%v", got, ok)
	}
}

func TestIssueOpenedSkippedWhenAssigned(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	cfg.IssueWatch.RequireUnassigned = true

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	h := NewHandler(cfg, store, nil)

	body := []byte(`{
		"action":"opened",
		"issue":{"number":1,"title":"x","state":"open","assignees":[{"login":"alice"}],"labels":[]},
		"repository":{"full_name":"o/r"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", signBody("s", body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["added"] != false {
		t.Fatalf("expected skip, got %v", resp)
	}
}

func TestPing(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	h := NewHandler(cfg, store, nil)

	body := []byte(`{"zen":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", signBody("s", body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ping status=%d", rec.Code)
	}
}

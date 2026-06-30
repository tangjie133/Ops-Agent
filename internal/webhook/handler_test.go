package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	var got Event
	h := NewHandler(cfg, store, func(evt Event) { got = evt })

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
	if got.Kind != EventAdded || got.Number != 42 {
		t.Fatalf("event: %+v", got)
	}
	item, ok := store.Get("o/r", 42)
	if !ok || item.Status != todo.StatusInTodo {
		t.Fatalf("store item: %+v ok=%v", item, ok)
	}
}

func TestIssueOpenedSkippedWhenAssigned(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	cfg.IssueWatch.RequireUnassigned = true

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	var got Event
	h := NewHandler(cfg, store, func(evt Event) { got = evt })

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
	if got.Kind != EventSkipped || got.Reason != "rule mismatch" {
		t.Fatalf("event: %+v", got)
	}
}

func TestIssueCommentEnqueuesOldIssue(t *testing.T) {
	cfg := config.Default()
	cfg.IssueWatch.Labels = nil
	cfg.IssueWatch.RequireUnassigned = false
	store, _ := todo.Load(t.TempDir() + "/todo.json")

	var got Event
	h := NewHandler(cfg, store, func(evt Event) { got = evt })

	body := []byte(`{
		"action":"created",
		"issue":{"number":55,"title":"legacy","state":"open","html_url":"https://github.com/o/r/issues/55","labels":[],"assignees":[]},
		"repository":{"full_name":"o/r"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got.Kind != EventCommentAdded || got.Number != 55 {
		t.Fatalf("event: %+v", got)
	}
}

func TestIssueClosedRemovesTodo(t *testing.T) {
	cfg := config.Default()
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	_ = store.Upsert(todo.Item{Repo: "o/r", Number: 7, Title: "active", Status: todo.StatusInTodo})

	var got Event
	h := NewHandler(cfg, store, func(evt Event) { got = evt })

	body := []byte(`{
		"action":"closed",
		"issue":{"number":7,"title":"active","state":"closed"},
		"repository":{"full_name":"o/r"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if got.Kind != EventClosed {
		t.Fatalf("event: %+v", got)
	}
	it, ok := store.Get("o/r", 7)
	if !ok || it.Status != todo.StatusDone {
		t.Fatalf("item=%+v ok=%v", it, ok)
	}
}

func TestPing(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	store, _ := todo.Load(t.TempDir() + "/todo.json")
	var got Event
	h := NewHandler(cfg, store, func(evt Event) { got = evt })

	body := []byte(`{"zen":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", signBody("s", body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ping status=%d", rec.Code)
	}
	if got.Kind != EventPing {
		t.Fatalf("event: %+v", got)
	}
}

func TestServerStartAndHealthz(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Listen = "127.0.0.1:0"
	cfg.Webhook.Secret = ""

	store, _ := todo.Load(t.TempDir() + "/todo.json")
	srv := NewServer(cfg, store, nil, nil)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	resp, err := http.Get(srv.HealthURL())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status=%d", resp.StatusCode)
	}
}

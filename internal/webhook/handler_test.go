package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func loadTasks(t *testing.T) *task.FileStore {
	t.Helper()
	s, err := task.Load(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIssueOpenedEnqueue(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "test-secret"
	tasks := loadTasks(t)
	var got Event
	h := NewHandler(cfg, tasks, func(evt Event) { got = evt }, nil)

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
	item, ok := tasks.FindIssue("o/r", 42)
	if !ok || item.Status != task.StatusCreated {
		t.Fatalf("task: %+v ok=%v", item, ok)
	}
}

func TestIssueOpenedDedupes(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "test-secret"
	tasks := loadTasks(t)
	h := NewHandler(cfg, tasks, func(evt Event) {}, nil)
	body := []byte(`{
		"action":"opened",
		"issue":{"number":7,"title":"bug","state":"open","html_url":"https://github.com/o/r/issues/7","labels":[],"assignees":[]},
		"repository":{"full_name":"o/r"}
	}`)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("X-Hub-Signature-256", signBody("test-secret", body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if n := len(tasks.List()); n != 1 {
		t.Fatalf("tasks=%d", n)
	}
}

func TestIssueCommentRequeues(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	tasks := loadTasks(t)
	_, tk, err := tasks.Ensure(task.Task{Repo: "o/r", Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue, SourceNumber: 55, Title: "old"})
	if err != nil {
		t.Fatal(err)
	}
	tk.Status = task.StatusDone
	if err := tasks.Update(tk); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(cfg, tasks, func(evt Event) {}, nil)
	body := []byte(`{
		"action":"created",
		"issue":{"number":55,"title":"old","state":"open","labels":[],"assignees":[]},
		"comment":{"body":"please retry","user":{"login":"alice"}},
		"repository":{"full_name":"o/r"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-Hub-Signature-256", signBody("s", body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "" {
		t.Fatal("empty body")
	}
	item, ok := tasks.FindIssue("o/r", 55)
	if !ok || item.Status != task.StatusDone {
		t.Fatalf("done task must not requeue on comment: %+v ok=%v", item, ok)
	}
}

func TestOwnAgentCommentDoesNotRequeue(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	tasks := loadTasks(t)
	_, tk, err := tasks.Ensure(task.Task{Repo: "o/r", Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue, SourceNumber: 56, Title: "old"})
	if err != nil {
		t.Fatal(err)
	}
	tk.Status = task.StatusDone
	if err := tasks.Update(tk); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(cfg, tasks, func(evt Event) {}, nil)
	body := []byte(`{
		"action":"created",
		"issue":{"number":56,"title":"old","state":"open","labels":[],"assignees":[]},
		"comment":{"body":"介绍库\n\n---\nHandled by Ops-Agent\nTask: t\nRun: r\nMode: auto","user":{"login":"tangjie133"}},
		"repository":{"full_name":"o/r"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-Hub-Signature-256", signBody("s", body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	item, ok := tasks.FindIssue("o/r", 56)
	if !ok || item.Status != task.StatusDone {
		t.Fatalf("task=%+v ok=%v", item, ok)
	}
}

func TestIssueClosedCancelsTask(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	tasks := loadTasks(t)
	if _, _, err := tasks.Ensure(task.Task{Repo: "o/r", Type: task.TypeIssueToPR, SourceKind: task.SourceKindIssue, SourceNumber: 12, Title: "x"}); err != nil {
		t.Fatal(err)
	}
	var got Event
	h := NewHandler(cfg, tasks, func(evt Event) { got = evt }, nil)
	body := []byte(`{"action":"closed","issue":{"number":12,"title":"x"},"repository":{"full_name":"o/r"}}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", signBody("s", body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got.Kind != EventClosed {
		t.Fatalf("event: %+v body=%s", got, rec.Body.String())
	}
	item, ok := tasks.FindIssue("o/r", 12)
	if !ok || item.Status != task.StatusCancelled {
		t.Fatalf("task=%+v ok=%v", item, ok)
	}
}

func TestPing(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = "s"
	var got Event
	h := NewHandler(cfg, nil, func(evt Event) { got = evt }, nil)

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

func TestIssueEventEnqueuesTask(t *testing.T) {
	store, err := task.Load(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Task.IssueToPR.Enabled = true
	h := NewHandler(cfg, store, nil, nil)
	body := []byte(`{"action":"opened","issue":{"number":45,"title":"bug","html_url":"http://example/45"},"repository":{"full_name":"o/r"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	items := store.List()
	if len(items) != 1 || items[0].SourceNumber != 45 || items[0].SourceKind != task.SourceKindIssue {
		t.Fatalf("tasks=%+v", items)
	}
}

func TestPullRequestEventEnqueuesTask(t *testing.T) {
	store, err := task.Load(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Task.IssueToPR.Enabled = true
	h := NewHandler(cfg, store, nil, nil)
	body := []byte(`{"action":"opened","pull_request":{"number":9,"title":"fix","html_url":"http://example/9"},"repository":{"full_name":"o/r"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	items := store.List()
	if len(items) != 1 || items[0].SourceNumber != 9 || items[0].SourceKind != task.SourceKindPR {
		t.Fatalf("status=%d tasks=%+v", rec.Code, items)
	}
}

func TestServerStartAndHealthz(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Listen = "127.0.0.1:0"
	cfg.Webhook.Secret = ""

	srv := NewServer(cfg, nil, nil, nil)
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

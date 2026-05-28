package webhook

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/issuewatch"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type OnEnqueue func(item todo.Item)

type Handler struct {
	cfg    *config.Config
	store  *todo.FileStore
	onAdd  OnEnqueue
	logger *log.Logger
}

func NewHandler(cfg *config.Config, store *todo.FileStore, onAdd OnEnqueue) *Handler {
	return &Handler{
		cfg:    cfg,
		store:  store,
		onAdd:  onAdd,
		logger: log.Default(),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	if h.cfg.Webhook.Secret != "" {
		if err := verifySignature(h.cfg.Webhook.Secret, body, r.Header.Get("X-Hub-Signature-256")); err != nil {
			h.logger.Printf("webhook: signature: %v", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		h.logger.Printf("webhook: warning — secret not set, skipping signature verify")
	}

	event := r.Header.Get("X-GitHub-Event")
	switch event {
	case "issues":
		h.handleIssues(w, body)
	case "ping":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"msg":"pong"}`))
	default:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"ignored":true}`))
	}
}

func (h *Handler) handleIssues(w http.ResponseWriter, body []byte) {
	var evt IssuesEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	// 第一步：仅 issue 创建时入待办（GitHub App / repo webhook 标准事件）
	if evt.Action != "opened" {
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}

	repo := evt.Repository.FullName
	ghIssue := evt.Issue.ToGitHubIssue()
	res, err := issuewatch.Enqueue(h.cfg, h.store, repo, ghIssue)
	if err != nil {
		h.logger.Printf("webhook: enqueue %s#%d: %v", repo, evt.Issue.Number, err)
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}

	if res.Added {
		h.logger.Printf("webhook: enqueued %s#%d %q", repo, res.Item.Number, res.Item.Title)
		if h.onAdd != nil {
			h.onAdd(res.Item)
		}
		writeJSON(w, map[string]any{"ok": true, "added": true, "repo": repo, "number": res.Item.Number})
		return
	}

	writeJSON(w, map[string]any{"ok": true, "added": false, "reason": res.Reason})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

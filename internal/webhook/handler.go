package webhook

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/issuewatch"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type Handler struct {
	cfg            *config.Config
	store          *todo.FileStore
	onEvent        OnEvent
	logger         *log.Logger
	secretWarnOnce sync.Once
}

func NewHandler(cfg *config.Config, store *todo.FileStore, onEvent OnEvent, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Handler{
		cfg:     cfg,
		store:   store,
		onEvent: onEvent,
		logger:  logger,
	}
}

func (h *Handler) emit(evt Event) {
	if h.onEvent != nil {
		h.onEvent(evt)
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
			h.logger.Printf("webhook · 签名校验失败: %v", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		h.secretWarnOnce.Do(func() {
			h.logger.Printf("webhook · 未设置 secret，跳过签名校验（本地调试）")
		})
	}

	event := r.Header.Get("X-GitHub-Event")
	switch event {
	case "issues":
		h.handleIssues(w, body)
	case "pull_request":
		h.handlePullRequest(w, body)
	case "issue_comment":
		h.handleIssueComment(w, body)
	case "ping":
		h.emit(Event{Kind: EventPing})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"msg":"pong"}`))
	default:
		h.emit(Event{Kind: EventIgnored, Reason: event})
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

	repo := evt.Repository.FullName
	number := evt.Issue.Number
	title := evt.Issue.Title

	switch evt.Action {
	case "opened":
		h.handleIssueOpened(w, repo, evt)
	case "closed", "deleted":
		h.handleIssueClosed(w, repo, number, title)
	case "reopened":
		h.handleIssueReopened(w, repo, evt)
	default:
		h.emit(Event{
			Kind:   EventSkipped,
			Repo:   repo,
			Number: number,
			Title:  title,
			Reason: "action=" + evt.Action,
		})
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
	}
}

func (h *Handler) handleIssueOpened(w http.ResponseWriter, repo string, evt IssuesEvent) {
	number := evt.Issue.Number
	title := evt.Issue.Title

	if !h.cfg.IssueWatch.Enabled {
		h.emit(Event{
			Kind:   EventSkipped,
			Repo:   repo,
			Number: number,
			Title:  title,
			Reason: "issue_watch disabled",
		})
		writeJSON(w, map[string]any{"ok": true, "added": false, "reason": "issue_watch disabled"})
		return
	}

	ghIssue := evt.Issue.ToGitHubIssue()
	res, err := issuewatch.Enqueue(h.cfg, h.store, repo, ghIssue)
	if err != nil {
		h.logger.Printf("webhook · 入队失败 %s#%d: %v", repo, number, err)
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}

	if res.Added {
		h.emit(Event{
			Kind:   EventAdded,
			Repo:   repo,
			Number: res.Item.Number,
			Title:  res.Item.Title,
		})
		writeJSON(w, map[string]any{"ok": true, "added": true, "repo": repo, "number": res.Item.Number})
		return
	}

	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: res.Reason,
	})
	writeJSON(w, map[string]any{"ok": true, "added": false, "reason": res.Reason})
}

func (h *Handler) handleIssueClosed(w http.ResponseWriter, repo string, number int, title string) {
	res, err := issuewatch.RemoveClosed(h.store, repo, number)
	if err != nil {
		h.logger.Printf("webhook · 关闭同步失败 %s#%d: %v", repo, number, err)
		http.Error(w, "sync failed", http.StatusInternalServerError)
		return
	}
	if res.Removed {
		h.emit(Event{Kind: EventClosed, Repo: repo, Number: number, Title: title})
		writeJSON(w, map[string]any{"ok": true, "removed": true, "repo": repo, "number": number})
		return
	}
	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: res.Reason,
	})
	writeJSON(w, map[string]any{"ok": true, "removed": false, "reason": res.Reason})
}

func (h *Handler) handleIssueReopened(w http.ResponseWriter, repo string, evt IssuesEvent) {
	number := evt.Issue.Number
	title := evt.Issue.Title
	ghIssue := evt.Issue.ToGitHubIssue()
	res, err := issuewatch.Reopen(h.cfg, h.store, repo, ghIssue)
	if err != nil {
		h.logger.Printf("webhook · 重开失败 %s#%d: %v", repo, number, err)
		http.Error(w, "reopen failed", http.StatusInternalServerError)
		return
	}
	if res.Added {
		h.emit(Event{
			Kind:   EventReopened,
			Repo:   repo,
			Number: res.Item.Number,
			Title:  res.Item.Title,
		})
		writeJSON(w, map[string]any{"ok": true, "added": true, "repo": repo, "number": res.Item.Number})
		return
	}
	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: res.Reason,
	})
	writeJSON(w, map[string]any{"ok": true, "added": false, "reason": res.Reason})
}

func (h *Handler) handlePullRequest(w http.ResponseWriter, body []byte) {
	var evt PullRequestEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	repo := evt.Repository.FullName
	number := evt.PullRequest.Number
	title := evt.PullRequest.Title

	switch evt.Action {
	case "closed":
		h.handleIssueClosed(w, repo, number, title)
	default:
		h.emit(Event{
			Kind:   EventSkipped,
			Repo:   repo,
			Number: number,
			Title:  title,
			Reason: "action=" + evt.Action,
		})
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
	}
}

func (h *Handler) handleIssueComment(w http.ResponseWriter, body []byte) {
	var evt IssueCommentEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	repo := evt.Repository.FullName
	number := evt.Issue.Number
	title := evt.Issue.Title

	if evt.Action != "created" {
		h.emit(Event{
			Kind:   EventSkipped,
			Repo:   repo,
			Number: number,
			Title:  title,
			Reason: "action=" + evt.Action,
		})
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}

	if !h.cfg.IssueWatch.Enabled {
		h.emit(Event{
			Kind:   EventSkipped,
			Repo:   repo,
			Number: number,
			Title:  title,
			Reason: "issue_watch disabled",
		})
		writeJSON(w, map[string]any{"ok": true, "added": false, "reason": "issue_watch disabled"})
		return
	}

	ghIssue := evt.Issue.ToGitHubIssue()
	res, err := issuewatch.EnqueueOnComment(h.cfg, h.store, repo, ghIssue)
	if err != nil {
		h.logger.Printf("webhook · 评论入队失败 %s#%d: %v", repo, number, err)
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}

	if res.Added {
		kind := EventCommentAdded
		h.emit(Event{
			Kind:   kind,
			Repo:   repo,
			Number: res.Item.Number,
			Title:  res.Item.Title,
		})
		writeJSON(w, map[string]any{"ok": true, "added": true, "repo": repo, "number": res.Item.Number})
		return
	}

	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: res.Reason,
	})
	writeJSON(w, map[string]any{"ok": true, "added": false, "reason": res.Reason})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

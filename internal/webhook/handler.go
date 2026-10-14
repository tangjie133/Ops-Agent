package webhook

// handler.go — GitHub Webhook HTTP 处理：签名校验、事件分发、入队待办/验收。

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/orchestrator"
	"github.com/ZzedJay/Ops-Agent/internal/task"
)

// Handler 处理 GitHub Webhook HTTP 请求，解析事件并更新任务/验收队列。
type Handler struct {
	cfg            *config.Config
	tasks          *task.FileStore
	onEvent        OnEvent
	logger         *log.Logger
	secretWarnOnce sync.Once
}

func NewHandler(cfg *config.Config, tasks *task.FileStore, onEvent OnEvent, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Handler{
		cfg:     cfg,
		tasks:   tasks,
		onEvent: onEvent,
		logger:  logger,
	}
}

func (h *Handler) emit(evt Event) {
	if h.onEvent != nil {
		h.onEvent(evt)
	}
}

// ServeHTTP 实现 http.Handler：校验签名、解析 payload、分发 issue/libtest 事件。
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
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
	}
}

func (h *Handler) handleIssueOpened(w http.ResponseWriter, repo string, evt IssuesEvent) {
	h.enqueueIssue(w, repo, evt, EventAdded)
}

func (h *Handler) handleIssueClosed(w http.ResponseWriter, repo string, number int, title string) {
	if h.tasks == nil {
		writeEnqueueSkip(w, "no task store")
		return
	}
	removed, err := h.tasks.CancelIssue(repo, number)
	if err != nil {
		h.logger.Printf("webhook · 关闭同步失败 %s#%d: %v", repo, number, err)
		http.Error(w, "sync failed", http.StatusInternalServerError)
		return
	}
	if removed {
		h.emit(Event{Kind: EventClosed, Repo: repo, Number: number, Title: title})
		writeJSON(w, map[string]any{"ok": true, "removed": true, "repo": repo, "number": number})
		return
	}
	writeRemoveSkip(w, "no active task")
}

func (h *Handler) handleIssueReopened(w http.ResponseWriter, repo string, evt IssuesEvent) {
	h.enqueueIssue(w, repo, evt, EventReopened)
}

func (h *Handler) enqueueIssue(w http.ResponseWriter, repo string, evt IssuesEvent, kind EventKind) {
	number := evt.Issue.Number
	if h.cfg == nil || !h.cfg.Task.IssueToPR.Enabled {
		writeEnqueueSkip(w, "issue_to_pr disabled")
		return
	}
	if h.tasks == nil {
		h.logger.Printf("webhook · task store 未接入 %s#%d", repo, number)
		http.Error(w, "task store missing", http.StatusInternalServerError)
		return
	}
	ghIssue := evt.Issue.ToGitHubIssue()
	created, tk, err := orchestrator.EnqueueIssue(h.cfg, h.tasks, repo, ghIssue, fmt.Sprintf("issues:%s:%d", evt.Action, number))
	if err != nil {
		h.logger.Printf("webhook · task 入队失败 %s#%d: %v", repo, number, err)
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}
	if !created {
		if _, tk, err = h.tasks.RequeueIssue(repo, number); err != nil {
			h.logger.Printf("webhook · task 重新入队失败 %s#%d: %v", repo, number, err)
			http.Error(w, "enqueue failed", http.StatusInternalServerError)
			return
		}
	}
	h.emit(Event{Kind: kind, Repo: repo, Number: tk.SourceNumber, Title: tk.Title})
	writeJSON(w, map[string]any{"ok": true, "added": true, "repo": repo, "number": tk.SourceNumber})
}

func (h *Handler) handlePullRequest(w http.ResponseWriter, body []byte) {
	var evt PullRequestEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	if evt.Action != "opened" {
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}
	repo := evt.Repository.FullName
	number := evt.PullRequest.Number
	title := evt.PullRequest.Title
	created, tk, err := orchestrator.EnqueuePR(h.cfg, h.tasks, repo, number, title, "")
	if err != nil {
		h.logger.Printf("webhook · pr 入队失败 %s#%d: %v", repo, number, err)
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}
	if !created {
		writeJSON(w, map[string]any{"ok": true, "added": false, "repo": repo, "number": tk.SourceNumber})
		return
	}
	h.emit(Event{Kind: EventAdded, Repo: repo, Number: tk.SourceNumber, Title: tk.Title})
	writeJSON(w, map[string]any{"ok": true, "added": true, "repo": repo, "number": tk.SourceNumber})
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
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}

	if skipOwnAutoReply(h.cfg, evt.Comment) {
		writeEnqueueSkip(w, "own auto reply")
		return
	}
	if h.cfg == nil || !h.cfg.Task.IssueToPR.Enabled {
		writeEnqueueSkip(w, "issue_to_pr disabled")
		return
	}

	_ = title
	ghIssue := evt.Issue.ToGitHubIssue()
	if h.tasks == nil {
		http.Error(w, "task store missing", http.StatusInternalServerError)
		return
	}
	created, tk, err := orchestrator.EnqueueIssue(h.cfg, h.tasks, repo, ghIssue, fmt.Sprintf("comment:%d:%d", number, len(evt.Comment.Body)))
	if err != nil {
		h.logger.Printf("webhook · 评论入队失败 %s#%d: %v", repo, number, err)
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}
	if !created && tk.Status != task.StatusCreated {
		writeEnqueueSkip(w, "already handled")
		return
	}
	h.emit(Event{Kind: EventCommentAdded, Repo: repo, Number: number, Title: tk.Title})
	writeJSON(w, map[string]any{"ok": true, "added": true, "repo": repo, "number": number})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

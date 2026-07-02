package webhook

// handler.go — GitHub Webhook HTTP 处理：签名校验、事件分发、入队待办/验收。

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/issueapproval"
	"github.com/ZzedJay/Ops-Agent/internal/issuewatch"
	"github.com/ZzedJay/Ops-Agent/internal/libtest"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// Handler 处理 GitHub Webhook HTTP 请求，解析事件并更新待办/验收队列。
type Handler struct {
	cfg            *config.Config
	store          *todo.FileStore
	libTest        *libtest.FileStore
	onEvent        OnEvent
	logger         *log.Logger
	secretWarnOnce sync.Once
}

func NewHandler(cfg *config.Config, store *todo.FileStore, libTest *libtest.FileStore, onEvent OnEvent, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Handler{
		cfg:     cfg,
		store:   store,
		libTest: libTest,
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
	case "push":
		h.handlePush(w, body)
	case "release":
		h.handleRelease(w, body)
	case "repository":
		h.handleRepository(w, body)
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
	number := evt.Issue.Number
	title := evt.Issue.Title

	if !h.cfg.IssueWatch.Enabled {
		writeEnqueueSkip(w, "issue_watch disabled")
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
	if silentEnqueueReason(res.Reason) {
		writeEnqueueSkip(w, res.Reason)
		return
	}
	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: res.Reason,
	})
	writeEnqueueSkip(w, res.Reason)
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
	if silentEnqueueReason(res.Reason) {
		writeRemoveSkip(w, res.Reason)
		return
	}
	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: res.Reason,
	})
	writeRemoveSkip(w, res.Reason)
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
	if silentEnqueueReason(res.Reason) {
		writeEnqueueSkip(w, res.Reason)
		return
	}
	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: res.Reason,
	})
	writeEnqueueSkip(w, res.Reason)
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
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}

	if skipOwnAutoReply(h.cfg, evt.Comment) {
		writeEnqueueSkip(w, "own auto reply")
		return
	}

	if !h.cfg.IssueWatch.Enabled {
		writeEnqueueSkip(w, "issue_watch disabled")
		return
	}

	if issueapproval.IsApprovePRComment(evt.Comment.Body) {
		h.handleApprovePR(w, repo, number, title)
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

	reason := res.Reason
	if silentEnqueueReason(reason) {
		writeEnqueueSkip(w, reason)
		return
	}

	h.emit(Event{
		Kind:   EventSkipped,
		Repo:   repo,
		Number: number,
		Title:  title,
		Reason: reason,
	})
	writeEnqueueSkip(w, reason)
}

func (h *Handler) handleApprovePR(w http.ResponseWriter, repo string, number int, title string) {
	if !h.cfg.IssueAutomation.RefactorPR.CommentApprovalEnabled() {
		writeEnqueueSkip(w, "refactor_pr comment approval disabled")
		return
	}
	res, err := issuewatch.ConfirmFixPR(h.store, repo, number)
	if err != nil {
		h.logger.Printf("webhook · /approve-pr 失败 %s#%d: %v", repo, number, err)
		http.Error(w, "confirm fix failed", http.StatusInternalServerError)
		return
	}
	if res.Confirmed {
		h.emit(Event{
			Kind:   EventFixConfirmed,
			Repo:   repo,
			Number: res.Item.Number,
			Title:  res.Item.Title,
		})
		writeJSON(w, map[string]any{"ok": true, "fix_confirmed": true, "repo": repo, "number": res.Item.Number})
		return
	}
	reason := res.Reason
	if silentEnqueueReason(reason) {
		writeEnqueueSkip(w, reason)
		return
	}
	h.emit(Event{Kind: EventSkipped, Repo: repo, Number: number, Title: title, Reason: reason})
	writeEnqueueSkip(w, reason)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

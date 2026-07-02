package webhook

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/libtest"
)

func (h *Handler) handlePush(w http.ResponseWriter, body []byte) {
	var evt PushEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	h.cfg.LibTest.Normalize()
	if !h.cfg.LibTest.Enabled || !h.cfg.LibTest.OnPush || h.libTest == nil {
		writeJSON(w, map[string]any{"ok": true, "skipped": "lib_test"})
		return
	}

	branch := strings.TrimPrefix(evt.Ref, "refs/heads/")
	defaultBranch := evt.Repository.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if branch != defaultBranch {
		writeJSON(w, map[string]any{"ok": true, "skipped": branch})
		return
	}

	added, err := libtestEnqueue(h, evt.Repository.FullName, branch, "push", "push "+branch)
	if err != nil {
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "queued": added})
}

func (h *Handler) handleRelease(w http.ResponseWriter, body []byte) {
	var evt ReleaseEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	h.cfg.LibTest.Normalize()
	if !h.cfg.LibTest.Enabled || !h.cfg.LibTest.OnRelease || h.libTest == nil {
		writeJSON(w, map[string]any{"ok": true, "skipped": "lib_test"})
		return
	}
	if evt.Action != "published" {
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}
	tag := evt.Release.TagName
	title := evt.Release.Name
	if title == "" {
		title = "release " + tag
	}
	added, err := libtestEnqueue(h, evt.Repository.FullName, tag, "release", title)
	if err != nil {
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "queued": added})
}

// handleRepository 处理 GitHub repository 事件（新建仓库 action=created）。
func (h *Handler) handleRepository(w http.ResponseWriter, body []byte) {
	var evt RepositoryEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	repo := evt.Repository.FullName
	if evt.Action != "created" {
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}
	h.cfg.LibTest.Normalize()
	if !h.cfg.LibTest.Enabled || h.libTest == nil {
		h.logger.Printf("webhook · 验收入队跳过 %s: lib_test 未启用", repo)
		writeJSON(w, map[string]any{"ok": true, "skipped": "lib_test disabled"})
		return
	}
	if !h.cfg.LibTest.OnRepoCreated {
		h.logger.Printf("webhook · 验收入队跳过 %s: on_repo_created 未启用", repo)
		writeJSON(w, map[string]any{"ok": true, "skipped": "on_repo_created disabled"})
		return
	}
	ref := evt.Repository.DefaultBranch
	if ref == "" {
		ref = "HEAD"
	}
	added, err := libtestEnqueue(h, repo, ref, "repository", "new repository")
	if err != nil {
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "queued": added})
}

func libtestEnqueue(h *Handler, repo, ref, trigger, title string) (bool, error) {
	added, err := libtest.Enqueue(h.libTest, h.cfg.LibTest, repo, ref, trigger, title)
	if err != nil {
		h.logger.Printf("webhook · 验收入队失败 %s@%s: %v", repo, ref, err)
		return false, err
	}
	if added {
		h.logger.Printf("webhook · 验收入队 %s@%s (%s)", repo, ref, trigger)
		h.emit(Event{Kind: EventLibTestQueued, Repo: repo, Reason: ref + " · " + trigger, Title: title})
	} else if err == nil {
		h.logger.Printf("webhook · 验收入队跳过 %s@%s: 已在队列或已忽略", repo, ref)
	}
	return added, nil
}

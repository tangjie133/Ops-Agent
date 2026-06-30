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

func (h *Handler) handleCreate(w http.ResponseWriter, body []byte) {
	var evt RepositoryEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	h.cfg.LibTest.Normalize()
	if !h.cfg.LibTest.Enabled || !h.cfg.LibTest.OnRepoCreated || h.libTest == nil {
		writeJSON(w, map[string]any{"ok": true, "skipped": "lib_test"})
		return
	}
	if evt.Action != "created" {
		writeJSON(w, map[string]any{"ok": true, "skipped": evt.Action})
		return
	}
	added, err := libtestEnqueue(h, evt.Repository.FullName, "HEAD", "create", "new repository")
	if err != nil {
		http.Error(w, "enqueue failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "queued": added})
}

func libtestEnqueue(h *Handler, repo, ref, trigger, title string) (bool, error) {
	added, err := libtest.Enqueue(h.libTest, h.cfg.LibTest, repo, ref, trigger, title)
	if err != nil {
		h.logger.Printf("webhook · 验收入队失败 %s: %v", repo, err)
		return false, err
	}
	if added {
		h.emit(Event{Kind: EventLibTestQueued, Repo: repo, Reason: ref + " · " + trigger, Title: title})
	}
	return added, nil
}

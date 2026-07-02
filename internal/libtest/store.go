package libtest

// store.go — 库验收队列 JSON 持久化与状态流转。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Status 表示库验收条目的生命周期阶段。
type Status string

const (
	StatusPending   Status = "pending"   // 等待验收 Worker
	StatusChecking  Status = "checking"  // 正在 clone/测试
	StatusPass      Status = "pass"      // 验收通过
	StatusFail      Status = "fail"      // 验收失败
	StatusDismissed Status = "dismissed" // 用户忽略
)

// Item 待验收库（整仓，非 Issue）。
type Item struct {
	Repo      string    `json:"repo"`
	Ref       string    `json:"ref"`
	Trigger   string    `json:"trigger"`
	Title     string    `json:"title"`
	Status    Status    `json:"status"`
	Workspace string    `json:"workspace,omitempty"`
	Report    string    `json:"report,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Key 生成 repo@ref 唯一键；ref 空时默认为 HEAD。
func Key(repo, ref string) string {
	if ref == "" {
		ref = "HEAD"
	}
	return fmt.Sprintf("%s@%s", repo, ref)
}

// FileStore 基于 JSON 的验收队列持久化；Webhook/LibTest Worker/TUI 共用。
type FileStore struct {
	path       string
	mu         sync.RWMutex
	items      map[string]Item
	lastReload time.Time
}

// Load 从磁盘加载验收队列；文件不存在时返回空 store。
func Load(path string) (*FileStore, error) {
	s := &FileStore{path: path, items: make(map[string]Item)}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	for _, it := range items {
		s.items[Key(it.Repo, it.Ref)] = it
	}
	return s, nil
}

func itemLess(a, b Item) bool {
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.Before(b.CreatedAt)
	}
	return Key(a.Repo, a.Ref) < Key(b.Repo, b.Ref)
}

func (s *FileStore) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		return itemLess(out[i], out[j])
	})
	return out
}

func (s *FileStore) Get(repo, ref string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[Key(repo, ref)]
	return it, ok
}

func (s *FileStore) Upsert(item Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	k := Key(item.Repo, item.Ref)
	if ex, ok := s.items[k]; ok {
		item.CreatedAt = ex.CreatedAt
	} else if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.Status == "" {
		item.Status = StatusPending
	}
	item.UpdatedAt = now
	s.items[k] = item
	return s.saveLocked()
}

func (s *FileStore) Transition(repo, ref string, to Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := Key(repo, ref)
	it, ok := s.items[k]
	if !ok {
		return fmt.Errorf("libtest item not found: %s", k)
	}
	it.Status = to
	it.UpdatedAt = time.Now().UTC()
	s.items[k] = it
	return s.saveLocked()
}

func (s *FileStore) SetReport(repo, ref, workspace, report string, ok bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := Key(repo, ref)
	it, okItem := s.items[k]
	if !okItem {
		return fmt.Errorf("libtest item not found: %s", k)
	}
	it.Workspace = workspace
	it.Report = report
	if ok {
		it.Status = StatusPass
	} else {
		it.Status = StatusFail
	}
	it.UpdatedAt = time.Now().UTC()
	s.items[k] = it
	return s.saveLocked()
}

func (s *FileStore) ShouldEnqueue(repo, ref string) bool {
	it, ok := s.Get(repo, ref)
	if !ok {
		return true
	}
	switch it.Status {
	case StatusDismissed:
		return false
	case StatusPending, StatusChecking:
		return false
	default:
		return true
	}
}

func (s *FileStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	items := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool {
		return itemLess(items[i], items[j])
	})
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Reload 从磁盘重新加载（供 TUI 轮询）。
func (s *FileStore) Reload() error {
	_, err := s.ReloadIfChanged()
	return err
}

// ReloadIfChanged 仅在文件 mtime 变化时重载。
func (s *FileStore) ReloadIfChanged() (changed bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return false, nil
	}
	fi, statErr := os.Stat(s.path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			if len(s.items) == 0 {
				return false, nil
			}
			s.items = make(map[string]Item)
			s.lastReload = time.Time{}
			return true, nil
		}
		return false, fmt.Errorf("stat libtest store: %w", statErr)
	}
	mod := fi.ModTime()
	if !mod.After(s.lastReload) {
		return false, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return false, fmt.Errorf("reload libtest store: %w", err)
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return false, fmt.Errorf("parse libtest store: %w", err)
	}
	fresh := make(map[string]Item, len(items))
	for _, it := range items {
		fresh[Key(it.Repo, it.Ref)] = it
	}
	s.items = fresh
	s.lastReload = mod
	return true, nil
}

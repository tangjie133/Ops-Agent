package todo

// store.go — 待办队列 JSON 持久化、状态流转与 mtime 轮询重载。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Status 表示待办条目在 Issue 自动化流程中的阶段。
type Status string

const (
	StatusInTodo    Status = "in_todo"    // 等待 Worker 分析
	StatusAnalyzing Status = "analyzing"  // Investigator 正在分析
	StatusReady     Status = "ready"      // 草稿就绪，等待用户确认发布（semi）
	StatusPosted    Status = "posted"     // 评论已发布
	StatusDone      Status = "done"       // 流程结束
	StatusDismissed Status = "dismissed"  // 用户忽略
	StatusFailed    Status = "failed"     // 分析或发布失败
)

// Item 对应一条 GitHub Issue 待办及其分析草稿。
type Item struct {
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Labels    []string  `json:"labels,omitempty"`
	Status    Status    `json:"status"`
	Draft     string    `json:"draft,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileStore 基于 JSON 文件的待办持久化；Webhook/Worker/TUI 共用，带读写锁。
type FileStore struct {
	path       string
	mu         sync.RWMutex
	items      map[string]Item
	lastReload time.Time
}

// Key 生成待办唯一键 owner/repo#number。
func Key(repo string, num int) string {
	return fmt.Sprintf("%s#%d", repo, num)
}

// Load 从磁盘加载待办 store；文件不存在时返回空 store。
func Load(path string) (*FileStore, error) {
	s := &FileStore{
		path:  path,
		items: make(map[string]Item),
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read todo store: %w", err)
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse todo store: %w", err)
	}
	for _, it := range items {
		s.items[Key(it.Repo, it.Number)] = it
	}
	return s, nil
}

func (s *FileStore) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].Number > out[j].Number
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (s *FileStore) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, it := range s.items {
		switch it.Status {
		case StatusDismissed, StatusDone:
			continue
		default:
			n++
		}
	}
	return n
}

func (s *FileStore) Get(repo string, num int) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[Key(repo, num)]
	return it, ok
}

func (s *FileStore) Has(repo string, num int) bool {
	_, ok := s.Get(repo, num)
	return ok
}

// ShouldEnqueue 判断扫描是否应写入待办。
// dismissed / done 不再入队；已在队列中的也不重复写入。
func (s *FileStore) ShouldEnqueue(repo string, num int) bool {
	it, ok := s.Get(repo, num)
	if !ok {
		return true
	}
	switch it.Status {
	case StatusDismissed, StatusDone:
		return false
	default:
		return false
	}
}

func (s *FileStore) Upsert(item Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	key := Key(item.Repo, item.Number)
	if existing, ok := s.items[key]; ok {
		item.CreatedAt = existing.CreatedAt
	} else {
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
	}
	if item.Status == "" {
		item.Status = StatusInTodo
	}
	item.UpdatedAt = now
	s.items[key] = item
	return s.saveLocked()
}

func (s *FileStore) Transition(repo string, num int, to Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := Key(repo, num)
	it, ok := s.items[key]
	if !ok {
		return fmt.Errorf("todo item not found: %s", key)
	}
	it.Status = to
	it.UpdatedAt = time.Now().UTC()
	s.items[key] = it
	return s.saveLocked()
}

func (s *FileStore) SetDraft(repo string, num int, draft string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := Key(repo, num)
	it, ok := s.items[key]
	if !ok {
		return fmt.Errorf("todo item not found: %s", key)
	}
	it.Draft = draft
	it.UpdatedAt = time.Now().UTC()
	s.items[key] = it
	return s.saveLocked()
}

func (s *FileStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
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
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
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

// Reload 从磁盘重新加载（供 TUI 轮询；与 webhook/worker 共用文件锁）。
func (s *FileStore) Reload() error {
	_, err := s.ReloadIfChanged()
	return err
}

// ReloadIfChanged 仅在文件 mtime 变化时重载，避免轮询时无谓 JSON 解析。
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
		return false, fmt.Errorf("stat todo store: %w", statErr)
	}
	mod := fi.ModTime()
	if !mod.After(s.lastReload) {
		return false, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return false, fmt.Errorf("reload todo store: %w", err)
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return false, fmt.Errorf("parse todo store: %w", err)
	}
	fresh := make(map[string]Item, len(items))
	for _, it := range items {
		fresh[Key(it.Repo, it.Number)] = it
	}
	s.items = fresh
	s.lastReload = mod
	return true, nil
}

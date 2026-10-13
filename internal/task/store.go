package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// FileStore 用 JSON 文件保存任务与执行记录。
type FileStore struct {
	path  string
	mu    sync.RWMutex
	tasks map[string]Task
	byID  map[string]string
	runs  map[string]Run
	clock func() time.Time
}

type fileDoc struct {
	Tasks []Task `json:"tasks"`
	Runs  []Run  `json:"runs"`
}

var idSeq atomic.Uint64

// Load 读取任务存储；文件不存在时返回空存储。
func Load(path string) (*FileStore, error) {
	s := &FileStore{
		path:  path,
		tasks: map[string]Task{},
		byID:  map[string]string{},
		runs:  map[string]Run{},
		clock: time.Now,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read task store: %w", err)
	}
	var doc fileDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse task store: %w", err)
	}
	for _, tk := range doc.Tasks {
		s.tasks[tk.Key] = tk
		s.byID[tk.ID] = tk.Key
	}
	for _, r := range doc.Runs {
		s.runs[r.Key] = r
	}
	return s, nil
}

// Ensure 按 TaskKey 创建或复用任务。
func (s *FileStore) Ensure(in Task) (bool, Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := MakeTaskKey(in.Repo, in.Type, in.SourceKind, in.SourceNumber)
	if cur, ok := s.tasks[key]; ok {
		return false, cur, nil
	}
	now := s.now()
	in.Key = key
	in.ID = newID(now)
	if in.Status == "" {
		in.Status = StatusCreated
	}
	in.CreatedAt = now
	in.UpdatedAt = now
	s.tasks[key] = in
	s.byID[in.ID] = key
	if err := s.saveLocked(); err != nil {
		return false, Task{}, err
	}
	return true, in, nil
}

// BeginRun 为任务开启一次执行。相同触发内容返回 skipped；已有活跃执行则报错。
func (s *FileStore) BeginRun(taskKey, triggerHash string) (Run, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.tasks[taskKey]
	if !ok {
		return Run{}, false, fmt.Errorf("task not found: %s", taskKey)
	}
	runKey := MakeRunKey(taskKey, triggerHash)
	if existing, ok := s.runs[runKey]; ok {
		return existing, true, nil
	}
	if cur.ActiveRunID != "" {
		return Run{}, false, fmt.Errorf("并行 Run 未结束")
	}
	now := s.now()
	run := Run{
		ID:          newID(now),
		TaskID:      cur.ID,
		Key:         runKey,
		TriggerHash: triggerHash,
		Status:      StatusCreated,
		CreatedAt:   now,
	}
	s.runs[runKey] = run
	cur.ActiveRunID = run.ID
	cur.UpdatedAt = now
	s.tasks[taskKey] = cur
	if err := s.saveLocked(); err != nil {
		return Run{}, false, err
	}
	return run, false, nil
}

// ClearActiveRun 结束当前活跃执行，允许后续新触发进入。
func (s *FileStore) ClearActiveRun(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.byID[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	cur := s.tasks[key]
	cur.ActiveRunID = ""
	cur.UpdatedAt = s.now()
	s.tasks[key] = cur
	return s.saveLocked()
}

// Update 写回任务字段，保留调用方给出的状态。
func (s *FileStore) Update(in Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[in.Key]; !ok {
		return fmt.Errorf("task not found: %s", in.Key)
	}
	in.UpdatedAt = s.now()
	s.tasks[in.Key] = in
	s.byID[in.ID] = in.Key
	return s.saveLocked()
}

// GetByKey 按任务键读取任务。
func (s *FileStore) GetByKey(key string) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tk, ok := s.tasks[key]
	return tk, ok
}

func (s *FileStore) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

func newID(now time.Time) string {
	return fmt.Sprintf("%s-%d", now.UTC().Format("20060102T150405.000000000"), idSeq.Add(1))
}

func (s *FileStore) saveLocked() error {
	doc := fileDoc{}
	for _, tk := range s.tasks {
		doc.Tasks = append(doc.Tasks, tk)
	}
	for _, r := range s.runs {
		doc.Runs = append(doc.Runs, r)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil && filepath.Dir(s.path) != "." {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

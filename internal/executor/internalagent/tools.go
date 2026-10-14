package internalagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type toolbox struct {
	root string
}

func newToolbox(root string) *toolbox {
	return &toolbox{root: root}
}

func (t *toolbox) run(ctx context.Context, a action) (string, error) {
	switch a.Action {
	case "list_dir":
		return t.listDir(a.Path)
	case "search_repo":
		return t.search(ctx, a.Query)
	case "read_file":
		return t.readFile(a.Path, a.StartLine, a.EndLine)
	case "edit_file":
		return t.editFile(a)
	default:
		return "", fmt.Errorf("不支持的 action %q", a.Action)
	}
}

func (t *toolbox) editFile(a action) (string, error) {
	full, err := t.resolve(a.Path)
	if err != nil {
		return "", err
	}
	_, statErr := os.Stat(full)
	exists := statErr == nil
	if exists {
		if a.Old == "" && a.New == "" {
			return "", fmt.Errorf("edit_file: 已有文件必须用 old/new")
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		src := string(data)
		n := strings.Count(src, a.Old)
		if n == 0 {
			return "", fmt.Errorf("edit_file: old 片段在 %s 中未找到", a.Path)
		}
		if n != 1 {
			return "", fmt.Errorf("edit_file: old 片段在 %s 中出现 %d 次", a.Path, n)
		}
		out := strings.Replace(src, a.Old, a.New, 1)
		if err := os.WriteFile(full, []byte(out), 0o644); err != nil {
			return "", err
		}
		return "updated " + a.Path, nil
	}
	if strings.TrimSpace(a.Content) == "" {
		return "", fmt.Errorf("edit_file: 新建文件须提供 content")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(a.Content), 0o644); err != nil {
		return "", err
	}
	return "created " + a.Path, nil
}

func (t *toolbox) resolve(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == "." {
		return t.root, nil
	}
	if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return "", fmt.Errorf("非法路径")
	}
	full := filepath.Join(t.root, filepath.FromSlash(rel))
	root := filepath.Clean(t.root)
	full = filepath.Clean(full)
	if full != root && !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界")
	}
	return full, nil
}

func (t *toolbox) listDir(rel string) (string, error) {
	dir, err := t.resolve(rel)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		name := e.Name()
		if name == ".git" {
			continue
		}
		if e.IsDir() {
			name += "/"
		}
		b.WriteString(name)
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return "(空目录)", nil
	}
	return b.String(), nil
}

func (t *toolbox) readFile(rel string, start, end int) (string, error) {
	full, err := t.resolve(rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > end {
		start = 1
	}
	var b strings.Builder
	fmt.Fprintf(&b, "file: %s (%d lines)\n", rel, len(lines))
	for i := start; i <= end && i <= len(lines); i++ {
		fmt.Fprintf(&b, "%4d| %s\n", i, lines[i-1])
	}
	return b.String(), nil
}

func (t *toolbox) search(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("empty query")
	}
	cmd := exec.CommandContext(ctx, "grep", "-R", "-n", "-I", "--exclude-dir=.git", "-m", "40", query, ".")
	cmd.Dir = t.root
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil && text == "" {
		return "no matches", nil
	}
	return text, nil
}

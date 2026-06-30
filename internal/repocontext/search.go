package repocontext

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var sourceExtensions = map[string]bool{
	".go": true, ".py": true, ".rs": true, ".js": true, ".ts": true, ".tsx": true,
	".jsx": true, ".java": true, ".kt": true, ".swift": true, ".rb": true, ".cs": true,
	".c": true, ".cc": true, ".cpp": true, ".cxx": true, ".h": true, ".hpp": true,
	".ino": true, ".md": true, ".yaml": true, ".yml": true, ".toml": true,
}

var skipDirNames = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, ".github": true, "testdata": true,
}

// SearchRepo 在仓库中搜索与 Issue 相关的源文件，按命中权重排序。
func SearchRepo(ctx context.Context, repoPath string, terms []string, maxFiles int) []string {
	if repoPath == "" || len(terms) == 0 || maxFiles <= 0 {
		return nil
	}

	scores := map[string]int{}
	for i, term := range terms {
		weight := 10 - i/3
		if weight < 1 {
			weight = 1
		}
		if strings.Contains(term, "_REG_") || strings.Contains(term, "::") {
			weight += 5
		}
		for _, rel := range grepRepo(ctx, repoPath, term) {
			scores[rel] += weight
		}
	}

	type scored struct {
		path  string
		score int
	}
	ranked := make([]scored, 0, len(scores))
	for p, s := range scores {
		ranked = append(ranked, scored{p, s})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].path < ranked[j].path
	})

	out := make([]string, 0, maxFiles)
	seen := map[string]struct{}{}
	for _, item := range ranked {
		if _, ok := seen[item.path]; ok {
			continue
		}
		seen[item.path] = struct{}{}
		out = append(out, item.path)
		if len(out) >= maxFiles {
			break
		}
	}

	// 匹配到 .cpp 时尽量带上同名 .h
	var extras []string
	for _, rel := range out {
		ext := filepath.Ext(rel)
		if ext != ".cpp" && ext != ".cc" && ext != ".cxx" && ext != ".c" {
			continue
		}
		header := strings.TrimSuffix(rel, ext) + ".h"
		if _, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(header))); err == nil {
			if _, ok := seen[header]; !ok {
				extras = append(extras, header)
				seen[header] = struct{}{}
			}
		}
	}
	out = append(out, extras...)
	if len(out) > maxFiles+4 {
		out = out[:maxFiles+4]
	}
	return out
}

func grepRepo(ctx context.Context, repoPath, term string) []string {
	if out := gitGrep(ctx, repoPath, term); len(out) > 0 {
		return out
	}
	return walkGrep(repoPath, term)
}

func gitGrep(ctx context.Context, repoPath, term string) []string {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "grep", "-l", "-I", "--fixed-strings", term)
	raw, err := cmd.Output()
	if err != nil {
		return nil
	}
	return filterSourcePaths(splitLines(string(raw)))
}

func walkGrep(repoPath, term string) []string {
	termLower := strings.ToLower(term)
	var matches []string
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirNames[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !sourceExtensions[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if len(data) > 256_000 {
			data = data[:256_000]
		}
		if strings.Contains(strings.ToLower(string(data)), termLower) {
			matches = append(matches, rel)
		}
		return nil
	})
	return filterSourcePaths(matches)
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, filepath.ToSlash(line))
		}
	}
	return out
}

func filterSourcePaths(paths []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, p := range paths {
		p = normalizeRelPath(p)
		if p == "" {
			continue
		}
		if !sourceExtensions[strings.ToLower(filepath.Ext(p))] {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

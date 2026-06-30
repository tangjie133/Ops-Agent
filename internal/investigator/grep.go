package investigator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func grepContent(ctx context.Context, repoPath, query string, maxHits int) []grepHit {
	if out := gitGrepLines(ctx, repoPath, query, maxHits); len(out) > 0 {
		return out
	}
	return walkGrepLines(repoPath, query, maxHits)
}

func gitGrepLines(ctx context.Context, repoPath, query string, maxHits int) []grepHit {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "grep", "-n", "-I", "--fixed-strings", query)
	raw, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseGrepOutput(string(raw), maxHits)
}

func parseGrepOutput(raw string, maxHits int) []grepHit {
	var out []grepHit
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.IndexByte(line, ':')
		if i <= 0 {
			continue
		}
		path := line[:i]
		rest := line[i+1:]
		j := strings.IndexByte(rest, ':')
		if j <= 0 {
			continue
		}
		lineNum, err := strconv.Atoi(rest[:j])
		if err != nil {
			continue
		}
		text := rest[j+1:]
		if !isSourcePath(path) {
			continue
		}
		out = append(out, grepHit{Path: filepath.ToSlash(path), Line: lineNum, Text: text})
		if len(out) >= maxHits {
			break
		}
	}
	return out
}

func walkGrepLines(repoPath, query string, maxHits int) []grepHit {
	q := strings.ToLower(query)
	var out []grepHit
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(out) >= maxHits {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil || !isSourcePath(rel) {
			return nil
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(line), q) {
				out = append(out, grepHit{Path: rel, Line: i + 1, Text: line})
				if len(out) >= maxHits {
					return nil
				}
			}
		}
		return nil
	})
	return out
}

func isSourcePath(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".py", ".rs", ".js", ".ts", ".tsx", ".jsx", ".java", ".kt", ".swift", ".rb", ".cs",
		".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".ino", ".md", ".yaml", ".yml", ".toml":
		return true
	default:
		return false
	}
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", ".github":
		return true
	default:
		return false
	}
}

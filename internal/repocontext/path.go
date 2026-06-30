package repocontext

import (
	"path/filepath"
	"strings"
)

func normalizeRelPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return filepath.ToSlash(filepath.Clean(p))
}

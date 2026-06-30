package tui

import (
	"os"
	"strings"
	"sync"
	"time"
)

const cwdRefreshInterval = 30 * time.Second

// viewCacheState 缓存 View 输出与 status bar 中的 cwd，避免每帧 syscall / lipgloss。
type viewCacheState struct {
	mu sync.Mutex

	dirty      bool
	cachedView string
	cacheW     int
	cacheH     int

	cachedHeader string
	headerW      int

	cwd       string
	cwdLoaded time.Time
}

func (m *Model) markDirty() {
	m.viewCache.mu.Lock()
	m.viewCache.dirty = true
	m.viewCache.mu.Unlock()
}

func (m *Model) cachedCWD() string {
	m.viewCache.mu.Lock()
	defer m.viewCache.mu.Unlock()
	if m.viewCache.cwd != "" && time.Since(m.viewCache.cwdLoaded) < cwdRefreshInterval {
		return m.viewCache.cwd
	}
	cwd, _ := os.Getwd()
	if len(cwd) > 36 {
		cwd = "…" + cwd[len(cwd)-33:]
	}
	m.viewCache.cwd = cwd
	m.viewCache.cwdLoaded = time.Now()
	return cwd
}

func (m *Model) invalidateViewCache() {
	m.viewCache.mu.Lock()
	m.viewCache.dirty = true
	m.viewCache.cachedView = ""
	m.viewCache.cachedHeader = ""
	m.viewCache.mu.Unlock()
}

func (m *Model) tryCachedView() (string, bool) {
	m.viewCache.mu.Lock()
	defer m.viewCache.mu.Unlock()
	if m.viewCache.dirty || m.viewCache.cachedView == "" {
		return "", false
	}
	if m.viewCache.cacheW != m.width || m.viewCache.cacheH != m.height {
		return "", false
	}
	return m.viewCache.cachedView, true
}

func (m *Model) storeCachedView(view string) {
	m.viewCache.mu.Lock()
	m.viewCache.cachedView = view
	m.viewCache.cacheW = m.width
	m.viewCache.cacheH = m.height
	m.viewCache.dirty = false
	m.viewCache.mu.Unlock()
}

func (m *Model) renderHeaderCached() string {
	m.viewCache.mu.Lock()
	if m.viewCache.cachedHeader != "" && m.viewCache.headerW == m.width {
		h := m.viewCache.cachedHeader
		m.viewCache.mu.Unlock()
		var b strings.Builder
		b.WriteString(h)
		b.WriteString(m.renderStatusBar())
		b.WriteString("\n\n")
		return b.String()
	}
	m.viewCache.mu.Unlock()

	var b strings.Builder
	b.WriteString(styleBanner.Render(bannerASCII))
	b.WriteString("\n")
	b.WriteString(styleWelcome.Render("Welcome to Ops-Agent!  /help  ·  /mode  ·  Ctrl+C: quit"))
	b.WriteString("\n\n")
	static := b.String()

	m.viewCache.mu.Lock()
	m.viewCache.cachedHeader = static
	m.viewCache.headerW = m.width
	m.viewCache.mu.Unlock()

	b.Reset()
	b.WriteString(static)
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n\n")
	return b.String()
}

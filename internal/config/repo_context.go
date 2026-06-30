package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// RepoContextConfig 控制分析 Issue 时是否拉取仓库并注入文件上下文。
type RepoContextConfig struct {
	Enabled       bool `yaml:"enabled"`
	MaxFileBytes  int  `yaml:"max_file_bytes"`  // 单文件上限
	MaxTotalBytes int  `yaml:"max_total_bytes"` // 注入 prompt 的总上限
	MaxSearchFiles int `yaml:"max_search_files"` // 符号搜索最多纳入的文件数
}

func (c *RepoContextConfig) Normalize() {
	if c.MaxFileBytes <= 0 {
		c.MaxFileBytes = 12_288 // 12 KiB
	}
	if c.MaxTotalBytes <= 0 {
		c.MaxTotalBytes = 64_000
	}
	if c.MaxSearchFiles <= 0 {
		c.MaxSearchFiles = 8
	}
}

func ReposCacheDir() string {
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "repos")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "repos")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "repos")
	}
	return "repos"
}

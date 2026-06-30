package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// RAGConfig 本地知识库（仓库格式规范、数据手册等），供 Issue 分析与 repo_validate 使用。
// 知识库目录结构：
//
//	<knowledge_dir>/
//	  standards/     仓库格式规范 (*.yaml / *.md)
//	  datasheets/    芯片数据手册 (*.md / *.txt)
//	  repos/         按仓库归档的补充文档 (owner/repo/…)
type RAGConfig struct {
	Enabled          *bool  `yaml:"enabled,omitempty"`
	KnowledgeDir     string `yaml:"knowledge_dir"`      // 空则 ~/.local/share/ops-agent/knowledge
	ReindexOnAnalyze *bool  `yaml:"reindex_on_analyze,omitempty"`
	DefaultStandard  string `yaml:"default_standard"`   // repo_validate 默认规范名
	InjectTopK       int    `yaml:"inject_top_k"`
	SearchTopK       int    `yaml:"search_top_k"`
	ChunkLines       int    `yaml:"chunk_lines"`
	ChunkOverlapLines int   `yaml:"chunk_overlap_lines"`
	MaxFileBytes     int    `yaml:"max_file_bytes"`
	MaxChunksTotal   int    `yaml:"max_chunks_total"`
}

func (c *RAGConfig) On() bool {
	if c == nil || c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

func (c *RAGConfig) ReindexOnAnalyzeOn() bool {
	if c == nil || c.ReindexOnAnalyze == nil {
		return true
	}
	return *c.ReindexOnAnalyze
}

func (c *RAGConfig) Normalize() {
	if c == nil {
		return
	}
	if c.InjectTopK <= 0 {
		c.InjectTopK = 4
	}
	if c.SearchTopK <= 0 {
		c.SearchTopK = 8
	}
	if c.ChunkLines <= 0 {
		c.ChunkLines = 48
	}
	if c.ChunkOverlapLines < 0 {
		c.ChunkOverlapLines = 0
	}
	if c.ChunkOverlapLines >= c.ChunkLines {
		c.ChunkOverlapLines = c.ChunkLines / 4
	}
	if c.MaxFileBytes <= 0 {
		c.MaxFileBytes = 512_000
	}
	if c.MaxChunksTotal <= 0 {
		c.MaxChunksTotal = 8000
	}
}

// KnowledgeDir 知识库根目录。
func KnowledgeDir(cfg RAGConfig) string {
	if d := cfg.KnowledgeDir; d != "" {
		return d
	}
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "knowledge")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "knowledge")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "knowledge")
	}
	return "knowledge"
}

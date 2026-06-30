package rag

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

const knowledgeIndexKey = "knowledge"

// EnsureKnowledgeLayout 创建知识库目录与示例规范（若不存在）。
func EnsureKnowledgeLayout(cfg config.RAGConfig) (string, error) {
	root := config.KnowledgeDir(cfg)
	for _, sub := range []string{"standards", "datasheets", "repos"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return root, err
		}
	}
	readme := filepath.Join(root, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		_ = os.WriteFile(readme, []byte(knowledgeReadme), 0o644)
	}
	example := filepath.Join(root, "standards", "arduino-library.yaml")
	if _, err := os.Stat(example); os.IsNotExist(err) {
		_ = os.WriteFile(example, []byte(exampleArduinoStandard), 0o644)
	}
	return root, nil
}

// EnsureKnowledgeIndex 索引知识库（standards / datasheets / repos），不索引克隆仓库源码。
func EnsureKnowledgeIndex(ctx context.Context, cfg config.RAGConfig, log Logger) (*Index, error) {
	cfg.Normalize()
	if !cfg.On() {
		return nil, nil
	}
	root, err := EnsureKnowledgeLayout(cfg)
	if err != nil {
		return nil, err
	}

	var existing *Index
	if old, err := loadIndex(knowledgeIndexKey); err == nil {
		existing = old
	} else {
		existing = newIndex(knowledgeIndexKey)
	}

	sources, err := collectKnowledgeSources(ctx, root, cfg.MaxFileBytes)
	if err != nil {
		return nil, err
	}

	needsRebuild := len(existing.Chunks) == 0
	if !needsRebuild {
		for src, meta := range sources {
			if existing.FileTimes[src] != meta.modUnix {
				needsRebuild = true
				break
			}
		}
		if !needsRebuild && len(existing.FileTimes) != len(sources) {
			needsRebuild = true
		}
	}

	if !needsRebuild {
		logf(log, "RAG 知识库缓存命中 · %d 片段", len(existing.Chunks))
		return existing, nil
	}

	logf(log, "RAG 重建知识库索引 (%d 文件) …", len(sources))
	idx := newIndex(knowledgeIndexKey)
	for src, meta := range sources {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		chunks, err := chunksFromFile(src, meta.path, cfg)
		if err != nil {
			continue
		}
		idx.Chunks = append(idx.Chunks, chunks...)
		idx.FileTimes[src] = meta.modUnix
		if len(idx.Chunks) >= cfg.MaxChunksTotal {
			idx.Chunks = idx.Chunks[:cfg.MaxChunksTotal]
			break
		}
	}
	idx.rebuildStats()
	if err := saveIndex(idx); err != nil {
		return idx, err
	}
	logf(log, "RAG 知识库就绪 · %d 片段 · 目录 %s", len(idx.Chunks), root)
	return idx, nil
}

func collectKnowledgeSources(ctx context.Context, root string, maxBytes int) (map[string]fileMeta, error) {
	out := map[string]fileMeta{}
	for _, sub := range []string{"standards", "datasheets", "repos"} {
		dir := filepath.Join(root, sub)
		if err := walkKnowledgeTree(ctx, root, dir, out, maxBytes); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func walkKnowledgeTree(ctx context.Context, root, dir string, out map[string]fileMeta, maxBytes int) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isKnowledgeFile(path) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		src := filepath.ToSlash(rel)
		addSource(out, src, path, maxBytes)
		return nil
	})
}

func isKnowledgeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".txt", ".yaml", ".yml", ".rst", ".json":
		return true
	default:
		return false
	}
}

// KnowledgeRoot 返回知识库路径（供 /status、文档）。
func KnowledgeRoot(cfg config.RAGConfig) string {
	return config.KnowledgeDir(cfg)
}

const knowledgeReadme = `# Ops-Agent 本地知识库

将以下内容放入本目录，分析 Issue 与 repo_validate 时会检索此处（**不会**索引 GitHub 克隆仓库的源码）。

## 目录

- standards/   仓库格式规范（YAML），供 repo_validate 与 Agent 参考
- datasheets/  芯片/模块数据手册（Markdown 或纯文本）
- repos/       按 owner/repo/ 存放某仓库的补充说明

## 示例

standards/arduino-library.yaml  — Arduino 库目录规范
datasheets/SD3031.md              — SD3031 RTC 寄存器摘要

修改文件后，下次分析 Issue 时会自动重建索引。
`

const exampleArduinoStandard = `# Arduino 库格式规范（示例）
name: arduino-library
description: DFRobot 风格 Arduino 库目录结构

required_files:
  - README.md
  - keywords.txt
  - library.properties

required_dirs:
  - examples

min_demos: 1
demo_dir: examples

readme_should_contain:
  - "## Installation"
  - "## Example"

notes: |
  - 源码通常在根目录或 src/
  - examples/ 下至少一个可编译示例
`

package rag

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// Logger 可选调试输出。
type Logger func(string)

type fileMeta struct {
	path    string
	modUnix int64
}

func addSource(out map[string]fileMeta, src, path string, maxBytes int) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	if info.Size() > int64(maxBytes) {
		return
	}
	out[src] = fileMeta{path: path, modUnix: info.ModTime().Unix()}
}

func chunksFromFile(source, path string, cfg config.RAGConfig) ([]Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > cfg.MaxFileBytes {
		data = data[:cfg.MaxFileBytes]
	}
	lines := strings.Split(string(data), "\n")
	return chunkLines(source, lines, cfg.ChunkLines, cfg.ChunkOverlapLines), nil
}

// FormatHits 供 Investigator 工具与 prompt 注入。
func FormatHits(hits []ScoredChunk) string {
	if len(hits) == 0 {
		return "no results"
	}
	var b strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&b, "--- hit %d score=%.2f %s:%d-%d ---\n%s\n\n",
			i+1, h.Score, h.Chunk.Source, h.Chunk.StartLine, h.Chunk.EndLine, strings.TrimSpace(h.Chunk.Text))
	}
	return strings.TrimSpace(b.String())
}

// PromptSection 从 Issue 文本检索知识库并格式化为 prompt 段落。
func PromptSection(idx *Index, issueText string, topK int) string {
	if idx == nil || topK <= 0 {
		return ""
	}
	query := extractQuery(issueText)
	if query == "" {
		return ""
	}
	hits := idx.Search(query, topK)
	if len(hits) == 0 {
		return ""
	}
	return "\n\n── 本地知识库 RAG（规范 / 数据手册，优先参考）──\n" + FormatHits(hits)
}

func extractQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len(text) > 1200 {
		text = text[:1200]
	}
	return text
}

func logf(log Logger, format string, args ...any) {
	if log == nil {
		return
	}
	log(fmt.Sprintf(format, args...))
}

// IndexUpdated 返回索引更新时间。
func IndexUpdated(idx *Index) time.Time {
	if idx == nil {
		return time.Time{}
	}
	return idx.UpdatedAt
}

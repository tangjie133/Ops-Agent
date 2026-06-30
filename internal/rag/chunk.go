package rag

import (
	"fmt"
	"strings"
)

// Chunk 索引中的一段文本。
type Chunk struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Text      string `json:"text"`
}

func chunkLines(source string, lines []string, chunkLines, overlap int) []Chunk {
	if len(lines) == 0 || chunkLines <= 0 {
		return nil
	}
	if overlap >= chunkLines {
		overlap = chunkLines / 4
	}
	step := chunkLines - overlap
	if step <= 0 {
		step = chunkLines
	}

	var out []Chunk
	for start := 0; start < len(lines); start += step {
		end := start + chunkLines
		if end > len(lines) {
			end = len(lines)
		}
		text := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(text) == "" {
			if end >= len(lines) {
				break
			}
			continue
		}
		out = append(out, Chunk{
			ID:        fmt.Sprintf("%s:%d-%d", source, start+1, end),
			Source:    source,
			StartLine: start + 1,
			EndLine:   end,
			Text:      text,
		})
		if end >= len(lines) {
			break
		}
	}
	return out
}

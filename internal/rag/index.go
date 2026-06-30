package rag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const bm25K1 = 1.5
const bm25B = 0.75

// Index 持久化 BM25 索引。
type Index struct {
	Repo      string            `json:"repo"`
	UpdatedAt time.Time         `json:"updated_at"`
	FileTimes map[string]int64  `json:"file_times"`
	Chunks    []Chunk           `json:"chunks"`
	termFreq  []map[string]int  `json:"-"`
	docLen    []float64         `json:"-"`
	idf       map[string]float64 `json:"-"`
	avgDocLen float64           `json:"-"`
}

func newIndex(repo string) *Index {
	return &Index{
		Repo:      repo,
		FileTimes: map[string]int64{},
	}
}

func (idx *Index) rebuildStats() {
	n := len(idx.Chunks)
	idx.termFreq = make([]map[string]int, n)
	idx.docLen = make([]float64, n)
	df := map[string]int{}

	for i, ch := range idx.Chunks {
		tf := map[string]int{}
		for _, t := range tokenize(ch.Text) {
			tf[t]++
		}
		idx.termFreq[i] = tf
		var dl float64
		for _, c := range tf {
			dl += float64(c)
		}
		idx.docLen[i] = dl
		for t := range tf {
			df[t]++
		}
	}

	var totalLen float64
	for _, dl := range idx.docLen {
		totalLen += dl
	}
	if n > 0 {
		idx.avgDocLen = totalLen / float64(n)
	}
	if idx.avgDocLen == 0 {
		idx.avgDocLen = 1
	}

	idx.idf = map[string]float64{}
	for t, freq := range df {
		idx.idf[t] = idfScore(n, freq)
	}
}

func idfScore(n, df int) float64 {
	if df <= 0 {
		return 0
	}
	return 1 + (float64(n-df)+0.5)/(float64(df)+0.5)
}

// Search 返回 BM25 得分最高的片段。
func (idx *Index) Search(query string, topK int) []ScoredChunk {
	if idx == nil || len(idx.Chunks) == 0 {
		return nil
	}
	if len(idx.termFreq) != len(idx.Chunks) {
		idx.rebuildStats()
	}
	if topK <= 0 {
		topK = 8
	}

	qTerms := tokenize(query)
	if len(qTerms) == 0 {
		return nil
	}

	scores := make([]float64, len(idx.Chunks))
	for _, qt := range qTerms {
		idf := idx.idf[qt]
		if idf == 0 {
			continue
		}
		for i, tf := range idx.termFreq {
			f := float64(tf[qt])
			if f == 0 {
				continue
			}
			dl := idx.docLen[i]
			denom := f + bm25K1*(1-bm25B+bm25B*dl/idx.avgDocLen)
			scores[i] += idf * (f * (bm25K1 + 1)) / denom
		}
	}

	type pair struct {
		i     int
		score float64
	}
	ranked := make([]pair, 0, len(scores))
	for i, s := range scores {
		if s > 0 {
			ranked = append(ranked, pair{i, s})
		}
	}
	sort.Slice(ranked, func(a, b int) bool {
		if ranked[a].score != ranked[b].score {
			return ranked[a].score > ranked[b].score
		}
		return ranked[a].i < ranked[b].i
	})
	if len(ranked) > topK {
		ranked = ranked[:topK]
	}

	out := make([]ScoredChunk, len(ranked))
	for j, p := range ranked {
		out[j] = ScoredChunk{Chunk: idx.Chunks[p.i], Score: p.score}
	}
	return out
}

type ScoredChunk struct {
	Chunk Chunk
	Score float64
}

func indexPath(repo string) string {
	base := configRAGDir()
	safe := strings.ReplaceAll(repo, "/", string(os.PathSeparator))
	return filepath.Join(base, safe, "index.json")
}

func configRAGDir() string {
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "rag")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "rag"
	}
	return filepath.Join(home, ".local", "share", "ops-agent", "rag")
}

func loadIndex(repo string) (*Index, error) {
	path := indexPath(repo)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	idx.rebuildStats()
	return &idx, nil
}

func saveIndex(idx *Index) error {
	if idx == nil {
		return fmt.Errorf("nil index")
	}
	path := indexPath(idx.Repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	idx.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

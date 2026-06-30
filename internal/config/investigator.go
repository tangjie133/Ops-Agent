package config

// InvestigatorConfig 多轮 Issue 调查 Agent 参数。
type InvestigatorConfig struct {
	MaxSteps          int `yaml:"max_steps"`
	MaxToolErrors     int `yaml:"max_tool_errors"`
	ReadFileMaxLines  int `yaml:"read_file_max_lines"`
	ReadFileMaxBytes  int `yaml:"read_file_max_bytes"`
	SearchMaxHits     int `yaml:"search_max_hits"`
	TotalContextBytes int `yaml:"total_context_bytes"`
}

func (c *InvestigatorConfig) Normalize() {
	if c.MaxSteps <= 0 {
		c.MaxSteps = 12
	}
	if c.MaxToolErrors <= 0 {
		c.MaxToolErrors = 3
	}
	if c.ReadFileMaxLines <= 0 {
		c.ReadFileMaxLines = 200
	}
	if c.ReadFileMaxBytes <= 0 {
		c.ReadFileMaxBytes = 16_384
	}
	if c.SearchMaxHits <= 0 {
		c.SearchMaxHits = 30
	}
	if c.TotalContextBytes <= 0 {
		c.TotalContextBytes = 96_000
	}
}

package config

// InvestigatorConfig 多轮 Issue 调查 Agent 参数。
type InvestigatorConfig struct {
	MaxSteps            int  `yaml:"max_steps"`
	MaxIssueComments    int  `yaml:"max_issue_comments"` // 分析时纳入的最近用户评论数（排除 Agent 回复）
	MaxToolErrors       int  `yaml:"max_tool_errors"`
	ReadFileMaxLines    int  `yaml:"read_file_max_lines"`
	ReadFileMaxBytes    int  `yaml:"read_file_max_bytes"`
	SearchMaxHits       int  `yaml:"search_max_hits"`
	TotalContextBytes   int  `yaml:"total_context_bytes"`
	WebSearchEnabled    *bool `yaml:"web_search_enabled,omitempty"`
	WebFetchEnabled     *bool `yaml:"web_fetch_enabled,omitempty"`
	FetchMaxBytes       int   `yaml:"fetch_max_bytes"`
	FetchTimeoutSec     int   `yaml:"fetch_timeout_sec"`
	WebSearchMaxResults int   `yaml:"web_search_max_results"`
}

func (c *InvestigatorConfig) WebSearchOn() bool {
	if c.WebSearchEnabled == nil {
		return true
	}
	return *c.WebSearchEnabled
}

func (c *InvestigatorConfig) WebFetchOn() bool {
	if c.WebFetchEnabled == nil {
		return true
	}
	return *c.WebFetchEnabled
}

func (c *InvestigatorConfig) Normalize() {
	if c.MaxSteps <= 0 {
		c.MaxSteps = 12
	}
	if c.MaxIssueComments <= 0 {
		c.MaxIssueComments = 5
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
	if c.FetchMaxBytes <= 0 {
		c.FetchMaxBytes = 48_000
	}
	if c.FetchTimeoutSec <= 0 {
		c.FetchTimeoutSec = 20
	}
	if c.WebSearchMaxResults <= 0 {
		c.WebSearchMaxResults = 8
	}
	// WebSearchEnabled / WebFetchEnabled default true when unset (zero value false)
	// Use pointer or explicit default in Default() - set in config.Default()
}

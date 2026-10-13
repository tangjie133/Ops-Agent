package budget

import (
	"fmt"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// Policy 是解析后的预算上限。
type Policy struct {
	MaxSteps    int
	MaxRetries  int
	MaxDuration time.Duration
	MaxCommands int
	MaxLLMCalls int
}

// FromConfig 把配置中的预算字符串解析成可比较的策略。
func FromConfig(cfg config.BudgetConfig) (Policy, error) {
	d, err := time.ParseDuration(cfg.MaxDuration)
	if err != nil {
		return Policy{}, fmt.Errorf("budget max_duration: %w", err)
	}
	return Policy{
		MaxSteps:    cfg.MaxSteps,
		MaxRetries:  cfg.MaxRetries,
		MaxDuration: d,
		MaxCommands: cfg.MaxCommands,
		MaxLLMCalls: cfg.MaxLLMCalls,
	}, nil
}

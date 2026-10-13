package feedback

import (
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

// Kind 是反馈分类结果。
type Kind string

const (
	KindFixCode     Kind = "fix_code"
	KindFixTest     Kind = "fix_test"
	KindFixConfig   Kind = "fix_config"
	KindIgnoreNoise Kind = "ignore_noise"
	KindWait        Kind = "wait"
	KindEscalate    Kind = "escalate"
)

// Signal 是一次待分类的外部信号。
type Signal struct {
	Source       string
	VerifyFailed bool
	Text         string
}

// Classify 按第一轮最小规则判断信号该如何处理。
func Classify(cfg config.FeedbackConfig, sig Signal) Kind {
	if sig.Source == "verifier" && sig.VerifyFailed {
		text := strings.ToLower(sig.Text)
		if strings.Contains(text, "demo") || strings.Contains(text, "test") || strings.Contains(text, "go test") {
			return KindFixTest
		}
		return KindFixCode
	}
	if sig.Source == "github_comment" && cfg.EscalateOnUnknownReview {
		return KindEscalate
	}
	return KindEscalate
}

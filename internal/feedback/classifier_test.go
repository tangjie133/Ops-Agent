package feedback

import (
	"testing"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestClassifyVerifierFailure(t *testing.T) {
	cfg := config.FeedbackConfig{EscalateOnUnknownReview: true}
	if Classify(cfg, Signal{Source: "verifier", VerifyFailed: true, Text: "go test failed"}) != KindFixTest {
		t.Fatal("expected fix_test")
	}
	if Classify(cfg, Signal{Source: "verifier", VerifyFailed: true, Text: "missing README"}) != KindFixCode {
		t.Fatal("expected fix_code")
	}
}

func TestClassifyUnknownCommentEscalates(t *testing.T) {
	cfg := config.FeedbackConfig{EscalateOnUnknownReview: true}
	if Classify(cfg, Signal{Source: "github_comment", Text: "please rethink"}) != KindEscalate {
		t.Fatal("expected escalate")
	}
}

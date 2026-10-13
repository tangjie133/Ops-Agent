package budget

import (
	"testing"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

func TestMeterBlocksWhenStepsExceeded(t *testing.T) {
	p, err := FromConfig(config.BudgetConfig{MaxSteps: 2, MaxRetries: 3, MaxDuration: "30m", MaxCommands: 20, MaxLLMCalls: 12})
	if err != nil {
		t.Fatal(err)
	}
	m := NewMeter(time.Unix(0, 0))
	m.NoteStep()
	if !m.Allow(p, time.Unix(1, 0)) {
		t.Fatal("first step allowed")
	}
	m.NoteStep()
	if m.Allow(p, time.Unix(2, 0)) {
		t.Fatal("third step must block")
	}
}

func TestMeterBlocksWhenDurationExceeded(t *testing.T) {
	p, err := FromConfig(config.BudgetConfig{MaxSteps: 12, MaxRetries: 3, MaxDuration: "1s", MaxCommands: 20, MaxLLMCalls: 12})
	if err != nil {
		t.Fatal(err)
	}
	m := NewMeter(time.Unix(0, 0))
	if m.Allow(p, time.Unix(2, 0)) {
		t.Fatal("duration exceeded")
	}
}

func TestFromConfigRejectsBadDuration(t *testing.T) {
	_, err := FromConfig(config.BudgetConfig{MaxSteps: 1, MaxDuration: "abc"})
	if err == nil {
		t.Fatal("expected error")
	}
}

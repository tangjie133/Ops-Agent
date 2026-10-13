package budget

import "time"

// Meter 记录一次任务已经消耗的资源。EstimatedCost 只记录，不参与阻断。
type Meter struct {
	Started       time.Time
	Steps         int
	Retries       int
	Commands      int
	LLMCalls      int
	EstimatedCost float64
}

// NewMeter 从任务开始时间创建计量器。
func NewMeter(started time.Time) *Meter {
	return &Meter{Started: started}
}

// Allow 判断下一次动作是否仍在预算内。
func (m *Meter) Allow(p Policy, now time.Time) bool {
	if m.Steps >= p.MaxSteps || m.Commands >= p.MaxCommands || m.LLMCalls >= p.MaxLLMCalls || m.Retries >= p.MaxRetries {
		return false
	}
	if now.Sub(m.Started) > p.MaxDuration {
		return false
	}
	return true
}

// NoteStep 记录一次编排步骤。
func (m *Meter) NoteStep() { m.Steps++ }

// NoteCommand 记录一次外部命令。
func (m *Meter) NoteCommand() { m.Commands++ }

// NoteLLM 记录一次模型调用。
func (m *Meter) NoteLLM() { m.LLMCalls++ }

// NoteRetry 记录一次失败后的重试。
func (m *Meter) NoteRetry() { m.Retries++ }

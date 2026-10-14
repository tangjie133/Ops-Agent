package task

import "time"

// Status 是任务或单次执行所处阶段。
type Status string

const (
	StatusCreated                 Status = "created"
	StatusCollectingContext       Status = "collecting_context"
	StatusPlanning                Status = "planning"
	StatusAwaitingApproval        Status = "awaiting_approval"
	StatusExecuting               Status = "executing"
	StatusVerifying               Status = "verifying"
	StatusEvaluatingSafety        Status = "evaluating_safety"
	StatusAwaitingHumanReview     Status = "awaiting_human_review"
	StatusPublishing              Status = "publishing"
	StatusLearning                Status = "learning"
	StatusDone                    Status = "done"
	StatusFailed                  Status = "failed"
	StatusCancelled               Status = "cancelled"
	StatusRolledBack              Status = "rolled_back"
	StatusNeedsManualIntervention Status = "needs_manual_intervention"
)

// Task 是一次可复用的自动化工作单元。
type Task struct {
	ID            string     `json:"id"`
	Key           string     `json:"key"`
	Type          Type       `json:"type"`
	Status        Status     `json:"status"`
	Repo          string     `json:"repo"`
	SourceKind    SourceKind `json:"source_kind"`
	SourceNumber  int        `json:"source_number"`
	Title         string     `json:"title,omitempty"`
	URL           string     `json:"url,omitempty"`
	Branch        string     `json:"branch,omitempty"`
	PRURL         string     `json:"pr_url,omitempty"`
	CommentPosted bool       `json:"comment_posted,omitempty"`
	StopReason    string     `json:"stop_reason,omitempty"`
	WorkspacePath string     `json:"workspace_path,omitempty"`
	ActiveRunID   string     `json:"active_run_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Run 是某个 Task 的一次触发执行。
type Run struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	Key         string    `json:"key"`
	TriggerHash string    `json:"trigger_hash"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// Type 是任务种类。第一轮只实现 issue_to_pr。
type Type string

// TypeIssueToPR 表示从 Issue 修复并创建 Pull Request。
const TypeIssueToPR Type = "issue_to_pr"

// SourceKind 标识任务来源对象。
type SourceKind string

// SourceKindIssue 表示来源是 GitHub Issue。
const SourceKindIssue SourceKind = "issue"

// SourceKindPR 表示来源是 GitHub Pull Request。
const SourceKindPR SourceKind = "pr"

package task

// Type 是任务种类。第一轮只实现 issue_to_pr。
type Type string

// TypeIssueToPR 表示从 Issue 修复并创建 Pull Request。
const TypeIssueToPR Type = "issue_to_pr"

// SourceKind 标识任务来源对象。
type SourceKind string

// SourceKindIssue 表示来源是 GitHub Issue。
const SourceKindIssue SourceKind = "issue"

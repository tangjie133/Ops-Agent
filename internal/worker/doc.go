// Package worker 实现 Issue 待办的自动分析流程。
//
// 按 manual / semi / full 模式决定是否自动分析、是否自动发布评论，
// 并遵守每小时评论上限等安全策略。
package worker

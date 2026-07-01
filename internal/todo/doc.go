// Package todo 提供 Issue 待办队列的 JSON 文件存储。
//
// 状态流转：in_todo → analyzing → ready/posted/done/failed/dismissed。
// TUI、Webhook、Worker 共用同一 FileStore，TUI 通过 mtime 轮询刷新。
package todo

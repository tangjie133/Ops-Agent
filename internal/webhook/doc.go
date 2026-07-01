// Package webhook 接收并处理 GitHub Webhook 事件。
//
// 负责签名校验、事件解析、Issue 入队（issuewatch）与验收入队（libtest），
// 以及本地 HTTP 服务与可选 smee 隧道转发。
package webhook

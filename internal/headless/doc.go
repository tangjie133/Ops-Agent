// Package headless 提供非 TTY / CI 环境下的运行入口。
//
// 支持 PR 检测、Issue 扫描、仅 Webhook 服务等模式，由 main 按环境变量分发。
package headless

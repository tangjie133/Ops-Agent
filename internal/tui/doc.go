// Package tui 提供基于 Bubble Tea 的终端交互界面。
//
// 主要职责：
//   - 待办 / 验收列表展示与快捷键操作
//   - Issue 自动化 Worker、库验收的后台任务调度（tea.Cmd）
//   - Webhook / AI / 代理 / 模式等配置菜单
//   - 轮询磁盘 store 刷新 UI，日志写入 tui.log（不在界面展示）
//
// 性能相关：View 缓存、合并 tick、忽略非键盘内部消息，避免事件洪峰导致卡顿。
package tui

# Ops-Agent 开发计划

> 版本 v0.1 · 对齐 [README](../README.md) 里程碑 M0–M4

## 1. 目标

交付 TUI-first 的 GitHub 运维工具：

- Issue：后台扫描 → 待办 → manual / semi / full 自动化
- PR：TUI 检测 + CI headless 门禁
- 通知：Slack / 飞书 / 钉钉
- AI：本地 Qwen（llama-server HTTP）

## 2. 里程碑

| 阶段 | 周期（参考） | 交付物 | 验收标准 |
|------|-------------|--------|----------|
| **M0** | 2–3 天 | 工程骨架、配置、双入口 | `go build` 成功；`OPS_AGENT_CI=1` 走 headless |
| **M1** | 5–7 天 | TUI 壳 + `gh` 封装 | TUI 运行；`/status` 显示 repo 与 gh 状态 |
| **M2** | 5–7 天 | PR 检测 + 通知 + Actions | CI 失败 `exit 1` + webhook |
| **M2.5** | 7–10 天 | Issue 扫描 + 待办 + Worker 骨架 | 待办持久化；TUI 扫描 |

> **实现顺序建议：M0 → M1 → M2.5 → M2 → M3**（Issue 监视是 TUI 主线，应先于 PR CI 门禁）。

## 3. M0 任务

- [x] `go mod init github.com/ZzedJay/Ops-Agent`
- [x] 目录结构（见 [DESIGN.md](./DESIGN.md)）
- [x] `cmd/ops-agent/main.go`：headless / TUI 分流
- [x] `internal/config`：YAML + 默认值
- [x] `.ops-agent.yaml.example`
- [x] `Makefile`

## 4. M1 任务

- [x] `internal/github`：`gh --json` 封装
- [x] `AuthStatus`, `RepoFromCwd`
- [x] Bubble Tea：banner / status / output / input
- [x] `/status` 命令
- [x] 启动检查（gh 必检，llama 警告）

## 5. M2 任务（待做）

- [x] `internal/prcheck` — `check` / `rules` / `report` 分层
- [x] `internal/notify` — Slack / 飞书 / 钉钉 + `FromAppConfig`
- [x] `internal/headless/pr.go` — 默认 CI 路径，与 issue scan 分离
- [x] TUI `/check` — 仅展示报告，不发 notify
- [x] `.github/workflows/pr-check.yml`

**M2 验收：**

1. TUI `/check` 对当前分支 PR 输出 pass/fail 报告
2. `OPS_AGENT_CI=1` + 失败 PR → webhook 推送 + exit 1
3. notify 三通道并行，单通道失败汇总 error
4. 检测规则 MVP：checks 全绿 + 无 merge conflict

## 6. M2.5 任务

- [x] `internal/issuewatch` — `filter` / `fetch` / `tui` / `ci` 分层
- [x] `internal/todo` — 持久化 + `ShouldEnqueue` 去重
- [x] `internal/worker` — M2.5 骨架（M3 前不自动推进）
- [x] TUI scanner — 仅 `ScanToTodo`，与 Worker 解耦
- [x] TUI 待办面板 — j/k 选中、i 详情、d 忽略
- [x] `/mode` / `M` 切换 manual / semi / full
- [x] headless issue scan — `ScanMatches` + notify，**不写 todo**

**M2.5 验收：**

1. TUI 启动后定时扫描，命中 issue 写入 `%AppData%/ops-agent/todo.json`
2. 左侧面板显示 `in_todo` 条目；`d` 可 dismiss 且不再入队
3. `i` 或 `/issue N` 可查看详情
4. semi/full 模式下条目保持 `in_todo`（不 fake ready）
5. `OPS_AGENT_ISSUE_SCAN=1` 只 notify，不创建/修改 todo 文件

## 7. M3 任务（待做）

- [ ] `internal/ai`（OpenAI 兼容 HTTP）
- [ ] `internal/agent`（JSON 工具协议）
- [ ] Worker semi/full 完整流水线
- [ ] TUI 确认模态框

## 8. M4 任务（待做）

- [ ] `Ctrl+G` 监控面板
- [ ] AI 流式输出
- [ ] Issue headless workflow
- [ ] `repo_validate` 占位

## 9. 默认决策

| 项 | 默认值 |
|----|--------|
| `issue_automation.mode` | `semi` |
| PR 检测 M2 | checks 全绿 + 无 merge conflict |
| todo 路径 | `%AppData%/ops-agent/todo.json` / `~/.local/share/ops-agent/todo.json` |
| Worker 并发 | 1 |

## 10. 分支策略

```text
main     ← 稳定
develop  ← 集成
feature/m0-init
feature/m1-tui
...
```

## 11. Sprint（M0+M1 第一周）

| 日 | 内容 |
|----|------|
| D1 | go mod、目录、main 分流、config |
| D2 | example yaml、Makefile |
| D3 | github.Client |
| D4 | Bubble Tea Model |
| D5 | banner + status + input |
| D6 | `/status`、启动检查 |
| D7 | test、文档 |

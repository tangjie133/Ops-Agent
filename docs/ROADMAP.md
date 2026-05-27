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
| **M2.5** | 7–10 天 | Issue 扫描 + 待办 + Worker 骨架 | 待办持久化；manual 模式 |
| **M3** | 10–15 天 | AI + Agent + semi/full | semi 草稿确认后发 comment |
| **M4** | 5 天 | 流式、监控、Issue workflow | 体验闭环 |

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

- [ ] `internal/prcheck`
- [ ] `internal/notify`（slack / feishu / dingtalk）
- [ ] `internal/headless` PR 检测实现
- [ ] TUI `/check`
- [ ] `.github/workflows/pr-check.yml`

## 6. M2.5 任务（待做）

- [ ] `internal/issuewatch`
- [ ] `internal/todo`
- [ ] `internal/worker`
- [ ] TUI 待办面板 + scanner goroutine
- [ ] `/mode` 切换 manual / semi / full

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

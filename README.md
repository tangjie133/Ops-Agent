# Ops-Agent

面向 GitHub 工作流的运维 TUI：通过终端交互界面处理 Issue、PR 提交与检测，并由本地 AI（Qwen + llama.cpp）以自然语言驱动 Agent。

**快速上手（安装、Webhook、快捷键、交接清单）见 [docs/USAGE.zh-CN.md](docs/USAGE.zh-CN.md)。**

> 本文档为需求与设计说明（草案），供评审讨论；实现进度以代码为准。

---

## 1. 产品定位

| 维度 | 说明 |
|------|------|
| **是什么** | 基于 `gh` 的 GitHub 运维 **TUI**，KIRO 式对话 + Agent 工具调用 |
| **不是什么** | 不做面向运维的子命令式 CLI（无 `ops-agent pr check` 这类手工敲命令的工作流） |
| **交互原则** | 运维在 TUI 里用自然语言或 `/` 快捷指令完成任务；复杂参数由 Agent 与配置承担 |
| **主要用户** | 本机运维 / 开发者（TUI + Agent） |
| **技术栈** | Go、Bubble Tea、Agent + `gh` CLI、llama.cpp（HTTP 服务）、本地 Qwen（GGUF） |

### 1.1 为何舍弃经典 CLI

| 对比 | 子命令 CLI | TUI（选定） |
|------|------------|-------------|
| 学习成本 | 需记命令与 flags | 对话即可，适合运维排障场景 |
| 上下文 | 每次手动拼参数 | 状态栏展示 repo / 模型 / 路径，会话可延续 |
| AI 协作 | 与 LLM 割裂 | 同一界面完成「说 → 看 → 确认 → 执行」 |
| 误操作 | 易漏 flag | 可预览后再确认执行 |

底层仍通过 `gh` 执行 GitHub 操作；**只是不再暴露一层给人用的 Cobra 子命令树**。

### 1.2 Issue 与 PR 能力对称

| 能力 | Issue | PR |
|------|-------|-----|
| **TUI 手动** | 指定 issue 分析 / 回复 / 建分支 | 对话触发检测、写描述、创建 |
| **TUI 后台** | 规则扫描 → **待办列表** → 自动化流水线 | — |
| **Headless（CI）** | **按需**规则扫描 → 仅通知 | PR 事件自动检测 → 通知 → `exit 1` |
| **AI** | 分析、生成评论草稿、（可选）自动回复 | 生成 PR 描述（预览确认） |

---

## 2. 运行方式

### 2.1 默认：TUI（唯一人机入口）

```bash
ops-agent
```

- ASCII Banner、欢迎语、状态栏（项目名 / 模型 / 自动化模式 / 当前路径）
- **待办列表面板**：后台扫描命中的 issue（见 §7）
- 输入框：自然语言（如「检查当前 PR」「处理 #45」）
- 可选 **`/` 快捷指令**（见 §6），供熟手快速触发
- 快捷键（规划）：`Enter` 发送、`M` 切换自动化模式、`Ctrl+G` Agent 监控、`Ctrl+C` 退出
- 启动前检查：`gh auth status`、本地 llama-server 是否可达（AI 功能依赖）

**TUI 内后台任务（进程内 goroutine，关 TUI 即停）：**

1. **Issue 扫描器**：按配置间隔 `gh issue list`，匹配规则后写入待办
2. **Issue Worker**：对待办条目按自动化模式执行分析 / 回复流水线

### 2.2 Headless 自动化（仅 CI，非运维界面）

GitHub Actions **无法**使用 TUI。同一二进制在检测到 CI / 非 TTY 时进入 **headless 模式**（不启动 Bubble Tea、不连接 llama-server）。

触发条件（满足其一即可）：

- 环境变量 `CI=true` 或 `OPS_AGENT_CI=1`
- 标准输出非终端（`!isatty`）

通过环境变量区分任务类型：

| 变量 | 行为 |
|------|------|
| （默认） | PR 检测 → 失败通知 → **`exit 1`** |
| `OPS_AGENT_ISSUE_SCAN=1` | Issue 规则扫描 → **仅通知** → 默认 `exit 0`（门禁场景可配置 `exit 1`） |

> Headless **不参与** Issue 的 AI 分析或自动回复；**待办列表仅由 TUI 维护**。

---

## 3. 功能范围

### 3.1 实现规划（MVP 分期）

| 模块 | 能力 | 阶段 |
|------|------|------|
| **TUI 壳** | Banner、状态栏、对话区、待办面板 | M1 |
| **`gh` 封装** | 统一 `--json` 调用 | M1 |
| **PR 检测** | TUI 对话触发；CI headless 自动检测 | M2 |
| **通知** | Slack / 飞书 / 钉钉并行推送 | M2 |
| **Issue 监视** | 后台扫描（label + 未指派）→ 待办列表 + 持久化 | M2.5 |
| **Issue 自动化** | 三档模式（手动 / 半自动 / 全自动）+ Worker | M2.5–M3 |
| **AI Agent** | llama-server + 工具调用；自然语言与 `/` 指令 | M3 |
| **PR 提交** | 创建/更新 PR，AI 生成 title/body | M3 |
| **TUI 增强** | `Ctrl+G` 监控、流式输出、`/feedback` | M4 |
| **库格式检测** | **仅占位** | 占位 |

### 3.2 明确不做（MVP）

- 面向运维的 **子命令式 CLI**
- 微信 / 企业微信通知（后续可加）
- GitHub Enterprise Server 专项适配（预留 `GH_HOST` 即可）
- 库格式/目录规范检测的具体规则
- GitHub Actions 内运行本地大模型或 Issue AI 回复
- Webhook / GitHub App 实时推送（后续可加；MVP 用轮询）

---

## 4. 硬性约束

| 约束 | 说明 |
|------|------|
| **必须依赖 `gh`** | 所有 GitHub 读写通过 `gh`（建议统一 `gh ... --json`） |
| **人机交互仅 TUI** | 运维不通过子命令操作；CI 走 headless |
| **PR 检测失败** | CI 并行通知 + **`exit 1`**（job 变红） |
| **Issue 待办** | 仅 TUI 后台扫描维护；CI 按需扫描只通知、不写待办 |
| **Issue 自动化** | 可配置半自动 / 全自动；全自动需护栏（见 §7.4） |
| **本地模型** | Qwen + **llama-server**（HTTP）；仅 TUI / Worker 使用 |
| **语言** | Go |
| **许可证** | GPL-3.0 |

---

## 5. 架构概览

```
┌──────────────────────────────────────────────────────────────────┐
│                         ops-agent (Go)                            │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ TUI (Bubble Tea)                                            │  │
│  │  对话 / 待办面板 / 模式切换 / 预览确认                         │  │
│  └────────────┬───────────────────────────────┬─────────────────┘  │
│               │                               │                    │
│       ┌───────▼───────┐               ┌───────▼───────┐            │
│       │ issue scanner │               │ issue worker  │            │
│       │ (定时 gh list)│──────────────▶│ analyze/reply │            │
│       └───────────────┘               └───────┬───────┘            │
│               │                               │                    │
│               ▼                               ▼                    │
│          todo store                      Agent + ai + github        │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Headless（CI）：prcheck | issuescan → notify → exit         │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────┬───────────────────────────────┬───────────────────┘
               │ HTTP                           │ exec
               ▼                                ▼
      ┌─────────────────┐              ┌───────────────┐
      │  llama-server   │              │  gh CLI       │
      │  (Qwen GGUF)    │              └───────┬───────┘
      └─────────────────┘                      ▼
                                        GitHub.com
               Headless / TUI ──Webhook──▶ Slack / 飞书 / 钉钉
```

### 5.1 目录结构（规划）

```
cmd/ops-agent/
internal/
  tui/          # Bubble Tea：唯一人机界面
  headless/     # CI：PR 检测 / Issue 按需扫描
  issuewatch/   # 扫描规则、gh issue list
  todo/         # 待办队列、持久化、状态机
  worker/       # Issue 自动化流水线
  agent/        # 对话循环、工具调度
  ai/           # HTTP 客户端（llama-server）
  github/       # 封装 gh
  prcheck/      # PR 规则
  notify/       # slack, feishu, dingtalk, multi
  config/
```

### 5.2 AI 集成策略

- **推荐**：独立进程 `llama-server` + Go OpenAI 兼容 HTTP 客户端
- **不推荐 MVP**：CGO 将 llama.cpp 编入二进制
- **Agent 工具协议**：Prompt + JSON 输出，**不依赖** OpenAI 原生 `function_calling`
- **PR / 半自动 Issue 写回**：TUI 内 **预览 → 确认** 后再 `gh` 写入

---

## 6. TUI 交互设计

### 6.1 界面分区

```text
┌─────────────────────────────────────────────────────────────┐
│  OPS-AGENT（Banner）                                          │
│  Welcome · /mode 切换自动化 · /feedback（占位）                 │
├─────────────────────────────────────────────────────────────┤
│  qwen2.5 · semi · repo · 待办 3          /path/to/repo        │  ← 状态栏
├──────────────────┬──────────────────────────────────────────┤
│ 待办              │  [对话与 Agent 输出，Markdown]              │
│ ● #45 ready      │                                          │
│ ○ #46 analyzing  │                                          │
│ ○ #47 in_todo    │                                          │
├──────────────────┴──────────────────────────────────────────┤
│  ask a question, or describe a task                    ctrl+g │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 自然语言示例

| 用户输入 | 行为 |
|----------|------|
| 处理 #45 | 设置焦点 issue，展示草稿或触发分析 |
| 分析 issue 12 | `github_issue_analyze` |
| 检查当前 PR | `github_pr_check` |
| 给这个 PR 写描述 | `github_pr_describe`，预览后确认 |
| 创建 PR | `github_pr_create`，逐步确认 |

### 6.3 `/` 快捷指令

| 指令 | 说明 |
|------|------|
| `/check` | 检测当前上下文 PR |
| `/issue <n>` | 聚焦某 issue |
| `/mode` | 切换 Issue 自动化模式（manual / semi / full） |
| `/describe` | 生成 PR 描述（预览） |
| `/status` | gh + llama-server 健康状态 |
| `/monitor` | Agent 工具调用面板（同 `Ctrl+G`） |
| `/feedback` | 反馈入口（占位） |

### 6.4 待办列表快捷键（规划）

| 键 | 行为 |
|----|------|
| `Tab` | 在待办与对话区间切换焦点 |
| `Enter` | 选中待办；`ready` 状态下确认发送评论 |
| `e` | 编辑当前草稿 |
| `d` | 忽略（dismiss）当前条目 |
| `M` | 循环切换 manual → semi → full |

### 6.5 确认流

| 操作类型 | semi / manual | full |
|----------|---------------|------|
| PR 写回 | 必须预览确认 | 必须预览确认 |
| Issue 评论 | 必须预览确认 | 自动发送（见 §7.4 护栏） |
| 读操作 | 直接展示 | 直接展示 |

---

## 7. Issue 监视、待办与自动化

### 7.1 总体流程

```text
TUI 后台扫描（label + 未指派）
    → 命中 issue 写入待办（去重）
    → Worker 按 automation.mode 处理：
         manual：仅列表，等人选中再分析
         semi：  自动分析 → ready（草稿）→ 人确认 → gh issue comment
         full：  自动分析 → 自动 gh issue comment
    → 更新状态：posted / done / dismissed / failed
```

**与旧方案对比：** 不再需要「人工提交才启动 AI」；`semi` / `full` 下扫描进待办后 **自动进入 Worker**。`semi` 保留 **回复前确认**；`full` 去掉确认环节。

### 7.2 MVP 扫描规则

| 规则 | 说明 |
|------|------|
| `state` | `open` |
| `labels` | 与配置列表 **有交集**（OR），如 `["ops", "needs-triage"]` |
| `assignees` | **为空**（未指派） |
| 去重 | 以 `(repo, number)` 为 key；已 `dismissed` / `done` 默认不再入队 |

扫描实现：`gh issue list --json number,title,labels,assignees,updatedAt`（标签过滤按 gh 能力组合）。

### 7.3 待办状态机

```text
in_todo → analyzing → ready → posted → done
              │          │        │
              └──────────┴────────┴──→ failed / dismissed
```

| 状态 | 含义 |
|------|------|
| `in_todo` | 扫描命中，等待 Worker |
| `analyzing` | 正在拉取 issue + LLM 分析 |
| `ready` | 草稿已生成，等待确认（semi） |
| `posted` | 已 `gh issue comment` |
| `done` | 用户标记完成 |
| `dismissed` | 用户忽略 |
| `failed` | API / 模型 / 发送失败，可重试 |

**本地持久化：** `~/.local/share/ops-agent/todo.json`（Windows：`%AppData%/ops-agent/todo.json`）。仅存 id、repo、status、草稿摘要与时间戳；详情仍 `gh issue view`。

### 7.4 自动化模式（主开关）

三档模式，可在 TUI 用 `/mode` 或 `M` 切换，并写回配置文件：

| 模式 | 值 | 扫描后进待办 | AI 分析 | 回复 issue |
|------|-----|-------------|---------|------------|
| **手动** | `manual` | ✓ | 人选中后 | 人确认后 |
| **半自动** | `semi` | ✓ | **自动** | **人确认后** |
| **全自动** | `full` | ✓ | **自动** | **自动** |

**运行时切换：** 只影响 **之后** 进入 `in_todo` 的条目；已在 `analyzing` 的按进入时模式跑完。

**全自动护栏（硬编码，不可关闭）：**

- 模型输出为空 / 过短 → 不发送，标 `failed`
- 评论长度上限（合理截断）
- 建议生产环境配置 `auto_reply.only_labels` 白名单
- 速率限制 `max_comments_per_hour`

### 7.5 配置示例

```yaml
# .ops-agent.yaml 或 ~/.config/ops-agent/config.yaml

issue_watch:
  enabled: true
  interval: 5m
  repo: owner/name          # 默认当前 git remote
  labels: ["ops", "needs-triage"]
  require_unassigned: true
  todo:
    max_items: 50

issue_automation:
  mode: semi                # manual | semi | full
  auto_analyze: true        # manual 时为 false
  confirm_before_reply: true  # semi 时为 true；full 时忽略

  auto_reply:               # mode=full 时生效
    only_labels: []         # 空=全部；生产建议如 ["ops-auto"]
    max_comments_per_hour: 10
    comment_footer: |
      ---
      _Posted by Ops-Agent (auto)_

  notify_on_ready: false    # semi：草稿就绪时通知人确认
  notify_on_posted: false   # full：发送后通知

notify:
  on_failure: true
  channels:
    slack:
      enabled: true
      webhook_url: ${SLACK_WEBHOOK}
    feishu:
      enabled: true
      webhook_url: ${FEISHU_WEBHOOK}
    dingtalk:
      enabled: true
      webhook_url: ${DINGTALK_WEBHOOK}

ai:
  provider: openai-compatible
  base_url: http://127.0.0.1:8080/v1
  model: qwen2.5-coder
  api_key: local

ci:
  pr_check_on_events: [pull_request]
  issue_scan:
    enabled: false          # 默认关；按需 workflow 打开
    fail_on_match: false    # true 时未指派 ops issue 存在则 exit 1
```

### 7.6 配置预设（运维习惯）

```yaml
# 保守：只列表，完全人工驱动
issue_automation: { mode: manual, auto_analyze: false }

# 推荐默认：自动分析，人点发送
issue_automation: { mode: semi, auto_analyze: true, notify_on_ready: true }

# 例行 triage：仅 ops-auto 标签全自动回复
issue_automation:
  mode: full
  auto_reply: { only_labels: ["ops-auto"], max_comments_per_hour: 20 }
```

---

## 8. PR 检测与通知

### 8.1 检测项（规划）

- PR 元数据（title、body、labels）
- 分支是否落后 base、是否有冲突
- Required reviews / approvals
- GitHub Actions checks 状态
- 可选规则：是否关联 issue（`Fixes #n`）等

### 8.2 失败行为

**TUI 内：** 展示失败列表；可提示是否发送通知。

**CI headless：**

1. 汇总失败项 → `Alert`
2. 并行推送 Slack / 飞书 / 钉钉
3. （可选）`gh pr comment` 留档
4. **`exit 1`**

### 8.3 Alert 字段

| 字段 | 说明 |
|------|------|
| `title` | 如 `[FAILED] PR #42 checks` |
| `repo` | `org/repo` |
| `pr_number` / `pr_url` | PR 标识与链接 |
| `failures` | 失败项列表 |
| `run_url` | Actions run 链接 |

---

## 9. 本地 AI（Qwen + llama.cpp）

> 仅 TUI / Issue Worker / Agent 使用；CI headless **不**连接模型。

### 9.1 部署

```bash
llama-server -m /path/to/qwen2.5-coder-q4_k_m.gguf --host 127.0.0.1 --port 8080
```

Go 侧通过 OpenAI 兼容 HTTP 调用；**不推荐** CGO 嵌入 llama.cpp。

### 9.2 Agent 工具（规划）

| 工具 | 说明 |
|------|------|
| `github_issue_view` | `gh issue view` |
| `github_issue_analyze` | 拉取 issue → LLM 分析 |
| `github_issue_comment` | 确认或全自动后 `gh issue comment` |
| `github_pr_describe` | LLM 写 PR body |
| `github_pr_check` | PR 规则检测 |
| `github_pr_create` | 包装 `gh pr create` |
| `notify_send` | 多通道告警 |
| `repo_validate` | 占位 |

### 9.3 Worker 并发

- Issue Worker 限制并发（建议同时 1 路分析），避免打满 llama-server
- 发送评论受 GitHub API 速率与 `max_comments_per_hour` 约束

---

## 10. GitHub Actions 示例

### 10.1 PR 检测（默认 headless）

```yaml
name: PR Check
on:
  pull_request:
    types: [opened, synchronize, reopened]

jobs:
  ops-check:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: cli/cli-action@v2
      - name: Install ops-agent
        run: echo "install ops-agent"   # TODO: release / go install
      - name: PR check and notify
        env:
          OPS_AGENT_CI: "1"
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SLACK_WEBHOOK: ${{ secrets.SLACK_WEBHOOK }}
          FEISHU_WEBHOOK: ${{ secrets.FEISHU_WEBHOOK }}
          DINGTALK_WEBHOOK: ${{ secrets.DINGTALK_WEBHOOK }}
        run: ops-agent
```

### 10.2 Issue 扫描（按需）

不跑默认定时；人工或事件触发。仅规则扫描 + 通知，**不** AI、**不**写 TUI 待办。

```yaml
name: Issue Scan
on:
  workflow_dispatch:
  issues:
    types: [opened, labeled]

jobs:
  issue-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: cli/cli-action@v2
      - name: Issue scan and notify
        env:
          OPS_AGENT_CI: "1"
          OPS_AGENT_ISSUE_SCAN: "1"
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SLACK_WEBHOOK: ${{ secrets.SLACK_WEBHOOK }}
        run: ops-agent
```

---

## 11. 依赖与环境

| 依赖 | 本机 TUI | Actions headless |
|------|----------|------------------|
| [GitHub CLI (`gh`)](https://cli.github.com/) | 必需 | 必需 |
| `llama-server` + Qwen GGUF | 必需（AI / Worker） | 不需要 |
| Go 1.22+ | 开发构建 | 开发构建 |

### 环境变量

| 变量 | 说明 |
|------|------|
| `OPS_AGENT_CI` | `1` → headless |
| `OPS_AGENT_ISSUE_SCAN` | `1` → Issue 扫描（非 PR 检测） |
| `CI` | 亦可触发 headless |
| `GH_TOKEN` | Actions 中由 `GITHUB_TOKEN` 注入 |
| `GH_HOST` | 预留 Enterprise Server |
| `SLACK_WEBHOOK` / `FEISHU_WEBHOOK` / `DINGTALK_WEBHOOK` | 通知 |
| `OPS_AGENT_CONFIG` | 配置文件路径 |

---

## 12. MVP 里程碑

| 阶段 | 交付物 | 状态 |
|------|--------|------|
| **M0** | Go 模块、配置、headless/TUI 分流 | 已完成 |
| **M1** | Bubble Tea 壳、`gh` 封装、`/status`、`/mode` | 已完成 |
| **M2** | headless PR 检测 + 三通道通知 + Actions PR workflow | 待做 |
| **M2.5** | Issue 后台扫描 + 待办列表 + 持久化 + Worker（manual/semi/full） | 待做 |
| **M3** | llama-server + Agent 工具；Issue/PR 对话；semi 确认流 | 待做 |
| **M4** | `Ctrl+G` 监控、流式输出、Issue 按需 Actions workflow | 待做 |
| **占位** | `repo_validate` → not implemented | — |

---

## 13. 待讨论 / 待确认

- [ ] PR 检测规则清单（是否强制关联 issue 等）
- [ ] 通知文案模板与各渠道卡片格式
- [ ] Qwen 具体 GGUF 与 `llama-server` 参数（GPU 层数、context）
- [ ] `done` 的 issue 被 reopen 后是否重新入队
- [ ] Issue headless `fail_on_match: true` 的使用场景
- [ ] Release 分发方式（多平台二进制）
- [ ] 多人运维时本地 todo 是否同步到 GitHub Project（二期）

---

## 14. 相关说明

### GitHub Enterprise

- **GitHub.com**：MVP 默认
- **Enterprise Server**：预留 `GH_HOST` + `gh auth login --hostname`

### 与 `gh` 的分工

| 组件 | 职责 |
|------|------|
| `gh` | 认证、GitHub API |
| `ops-agent` | TUI、待办、Issue/PR 自动化、Agent、CI headless |

---

## 15. 开发

**要求：** Go 1.22+、[GitHub CLI](https://cli.github.com/)（`gh`）；AI 功能另需本地 `llama-server`。

```bash
git clone https://github.com/ZzedJay/Ops-Agent.git
cd Ops-Agent
go mod tidy
go build -o ops-agent.exe ./cmd/ops-agent

# 启动 TUI（默认）
./ops-agent.exe

# 复制配置示例
cp .ops-agent.yaml.example .ops-agent.yaml

# Headless（CI 占位，M2 实现 PR 检测）
set OPS_AGENT_CI=1
ops-agent.exe
```

**文档：** [docs/ROADMAP.md](docs/ROADMAP.md) · [docs/DESIGN.md](docs/DESIGN.md)

**当前进度（M0/M1）：** TUI 壳、`gh` 封装、`/status`、`/mode`、headless 分流；待办与 PR 检测见 ROADMAP。

---

## License

GPL-3.0 — see [LICENSE](LICENSE).

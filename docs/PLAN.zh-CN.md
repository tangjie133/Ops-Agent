# Ops-Agent 后续功能计划

> 记录日期：2026-06-30 · 供后续迭代对照，避免遗忘  
> 当前代码基线：Issue 多轮 Investigator、Webhook 待办、本地知识库 RAG、`repo_validate` 骨架

---

## 一、当前已完成（无需重复做）

| 模块 | 状态 | 说明 |
|------|------|------|
| Issue 流水线 | ✅ | Webhook → 待办 → 克隆仓库 → Investigator → 草稿 → semi 确认 / full 发布 |
| Investigator 工具 | ✅ | `search_repo` / `read_file` / `list_dir` / `web_search` / `fetch_url` / `rag_search` / `repo_validate` |
| 本地知识库 RAG | ✅ 骨架 | 索引 `knowledge/` 下 standards / datasheets / repos，**不**索引克隆源码 |
| `repo_validate` | ✅ MVP | `standards/*.yaml`：required_files / dirs、README 片段检查 |
| 调试 | ✅ | Investigator 日志 → TUI 日志区；`Ctrl+Y` 复制全部日志 |
| 代理 | ✅ | `/proxy` 菜单，gh / git / HTTP 共用 |

---

## 二、你需要手动填充的知识库（非代码）

路径默认：`~/.local/share/ops-agent/knowledge/`（或 `ai.rag.knowledge_dir`）

```
knowledge/
├── standards/           ← 仓库格式 YAML（首次运行已有 arduino-library.yaml 示例）
├── datasheets/          ← 芯片手册 Markdown/文本（如 SD3031.md）
├── repos/               ← 按 owner/repo/ 的补充说明
└── README.md
```

**待办清单：**

- [ ] 编写 `datasheets/SD3031.md`（CTR2、SQW 等寄存器摘要）
- [ ] 按 DFRobot 库实际结构完善 `standards/arduino-library.yaml`
- [ ] 为常用仓库添加 `repos/<owner>/<repo>/notes.md`
- [ ] 在 `.ops-agent.yaml` 设置 `ai.rag.default_standard`

---

## 三、功能 backlog（建议实现顺序）

### 阶段 A — 知识库与 Issue 质量（优先）

| # | 功能 | 目标 | 关键文件 / 入口 |
|---|------|------|-----------------|
| A1 | **PDF 入库** | 把 datasheet PDF 转成文本写入 `datasheets/` 并参与 RAG | `internal/rag/`、`fetch_url` |
| A2 | **知识库 TUI 菜单** | `/knowledge` 或 `/rag`：查看路径、重建索引、列出 standards | `internal/tui/` |
| A3 | **`/status` 展示 RAG** | 知识库路径、chunk 数、上次索引时间 | `internal/tui/commands.go` |
| A4 | **硬件 Issue 启发式** | 检测芯片型号 / `_REG_` 时强制 `rag_search` + 提示补手册 | `internal/investigator/investigator.go` |
| A5 | **草稿编辑 `e` 键** | semi 模式下编辑 ready 草稿再发布 | `internal/tui/model.go`、README §6.4 |
| A6 | **流式 AI 输出** | 分析过程逐字显示，减少「假死」感 | `internal/ai/client.go`、TUI |

**验收：** SD3031 类 Issue 能在日志看到 RAG 命中 datasheet；无手册时回复明确提示路径。

---

### 阶段 B — 仓库格式检测与测试（RAG 延伸）

| # | 功能 | 目标 | 关键文件 / 入口 |
|---|------|------|-----------------|
| B1 | **规范增强** | YAML 支持：`min_examples`、`forbidden_globs`、文件命名规则 | `internal/repovalidate/standard.go` |
| B2 | **规范内嵌测试命令** | `tests: [{name, cmd, expect_exit: 0}]` 分析或 CI 时执行 | `internal/repovalidate/` 新 `run_tests.go` |
| B3 | **TUI `/validate`** | 对选中待办仓库跑 `repo_validate`，结果进对话区 | `internal/tui/commands.go` |
| B4 | **Headless `repo_validate`** | `OPS_AGENT_VALIDATE=1` + 克隆路径 → exit 1 on fail | `internal/headless/` |
| B5 | **GitHub Actions 示例** | workflow：checkout → ops-agent validate → 通知 | `.github/workflows/` |

**验收：** DFRobot 库缺 `library.properties` 时 `/validate` 报 FAIL；规范里定义的 `pio run` 可自动跑。

---

### 阶段 C — 半自动修代码 + PR（较大）

| # | 功能 | 目标 | 关键文件 / 入口 |
|---|------|------|-----------------|
| C1 | **`edit_file` 工具** | Agent 在克隆目录写 patch（限 sandbox） | `internal/investigator/tools.go` |
| C2 | **`run_cmd` 工具** | 白名单：build / test / git status（禁任意 shell） | 同上 + 配置 `investigator.allowed_commands` |
| C3 | **Diff 预览** | TUI 模态框展示 patch，确认后再写盘 | `internal/tui/` |
| C4 | **git commit + push** | 确认后：分支、`git commit`、`git push` | `internal/github/` 或 `internal/git/` |
| C5 | **`gh pr create`** | 预览 title/body → 确认开 PR | 新 `internal/pr/` 或 agent 工具 |
| C6 | **与知识库联动** | 修复需符合 `standards/`；PR 描述引用 RAG 片段 | Investigator prompt |

**原则：** semi 必须「预览 → 确认」；full 仅 Issue 评论可全自动，**代码/PR 禁止全自动 push**。

**验收：** 选中 Issue → 分析 → 生成 patch → 预览 → 确认 → 出现 PR 链接。

---

### 阶段 D — PR / CI / M4 收尾

| # | 功能 | 目标 |
|---|------|------|
| D1 | **`/describe`** | AI 生成 PR body，预览确认 |
| D2 | **`Ctrl+G` 监控面板** | 实时看 Investigator 步骤（M4） |
| D3 | **`/feedback`** | 用户反馈入库或导出 |
| D4 | **Issue headless workflow** | Actions 按需扫描 + 通知 |
| D5 | **更新 ROADMAP / USAGE** | 与本文档和 README 对齐 |

---

## 四、推荐实施路线（一张图）

```text
现在
 │
 ├─► [你] 填充 knowledge/datasheets + standards
 │
 ▼
阶段 A（Issue 质量）
 │   PDF 入库 · TUI /knowledge · 草稿编辑 · 流式输出
 ▼
阶段 B（格式 + 测试）
 │   规范 YAML 增强 · /validate · CI headless
 ▼
阶段 C（修代码 + PR）
 │   edit_file · run_cmd · diff 预览 · gh pr create
 ▼
阶段 D（M4  polish）
     Ctrl+G · /describe · /feedback · 文档
```

---

## 五、配置速查（已有）

```yaml
ai:
  rag:
    enabled: true
    knowledge_dir: ""              # 空 = ~/.local/share/ops-agent/knowledge
    reindex_on_analyze: true
    default_standard: arduino-library
    inject_top_k: 4
    search_top_k: 8
  repo_context:
    enabled: true                  # 克隆仓库（Investigator 必需）
  investigator:
    web_search_enabled: true
    web_fetch_enabled: true

proxy:
  enabled: true
  https_proxy: http://127.0.0.1:7890
```

---

## 六、相关路径

| 用途 | 路径 |
|------|------|
| 知识库 | `~/.local/share/ops-agent/knowledge/` |
| RAG 索引 | `~/.local/share/ops-agent/rag/knowledge/index.json` |
| 克隆仓库 | `~/.local/share/ops-agent/repos/<owner>/<repo>/` |
| TUI 日志 | `~/.local/share/ops-agent/logs/tui.log` |
| 待办 | `~/.local/share/ops-agent/todo.json` |

---

## 七、决策待定（实现前需拍板）

- [ ] 默认 `default_standard` 用 `arduino-library` 还是按仓库 label 映射多规范？
- [ ] PDF 入库：本地 `pdftotext` 还是纯 Go 库？
- [ ] 阶段 C 分支策略：固定 `ops-agent/fix-issue-N` 还是 Agent 起名？
- [ ] 测试命令在克隆环境跑（需 PlatformIO/Arduino CLI）还是仅静态检查？

---

## 八、单任务 Issue 模板（复制到 GitHub 用）

```markdown
## 目标
（例如：A2 知识库 TUI 菜单）

## 验收
- [ ] …

## 参考
- docs/PLAN.zh-CN.md 阶段 A2
- internal/rag/knowledge.go
```

---

*完成某项后在本文件对应 `- [ ]` 改为 `- [x]`，并注明日期。*

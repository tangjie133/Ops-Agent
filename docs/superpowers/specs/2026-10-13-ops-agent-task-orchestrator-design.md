# Ops-Agent 任务编排架构重设计

> 日期：2026-10-13
> 状态：设计已确认，等待实现计划
> 范围：重新设计 Ops-Agent 的 Issue/PR 自动处理、知识库学习、新库验收、测试验证、回踩与回滚判断框架。

## 1. 背景与目标

当前项目已经具备 GitHub CLI 封装、Webhook、Issue 待办、RAG、重构 Agent、仓库规范检测和 demo 检测等模块，但职责边界不够清晰。后续要支持自动回复 Issue、处理 PR、自动学习知识库、处理新库格式检测与修改、完善 demo 与测试、调用 Arduino CLI、自动回踩和回滚判断，因此需要以统一任务编排核心重新设计。

目标是采用“任务编排核心 + 插件执行器”的架构：

- Issue、PR、新库验收、知识学习都统一建模为 Task。
- 内置 Agent 作为默认执行器，复杂任务可委托外部插件或 CLI。
- GitHub 写回、验证、风险判断、知识沉淀统一收口，避免散落在业务模块中。
- 高风险动作必须可审计、可阻断、可人工接管。
- 后续实现代码需要完整中文注释，尤其是核心状态机、策略判断和安全护栏。

## 2. 总体架构

核心流程：

```text
GitHub 事件 / TUI 指令
    → Task 创建
    → 上下文收集
    → 生成执行计划
    → 执行修改或回复
    → 验证测试
    → 风险评估与回踩
    → 写回 GitHub / 创建 PR
    → 经验沉淀到知识库
```

建议模块边界：

```text
internal/github       # 只封装 gh/GitHub 读写，不放业务决策
internal/task         # 统一任务模型、状态机、队列、持久化
internal/orchestrator # 编排 Task 每个阶段如何流转
internal/executor     # 执行器接口：内置 Agent、插件、CLI、人工接管
internal/verifier     # go test、repo_validate、arduino-cli、demo 检测
internal/safety       # diff 评估、回踩、回滚建议、人工确认策略
internal/knowledge    # 自动学习、知识分层、检索注入
internal/memory       # Task 运行日志、事件、结果
internal/publisher    # GitHub 写回策略与动作
internal/skill        # 稳定工作流定义和护栏
internal/tui          # 人工审批、接管、查看日志
internal/headless     # 后台/CI 运行入口
internal/webhook      # GitHub 事件入口
```

旧模块迁移方向：

- `rag` 升级为 `knowledge` 的检索层。
- `refactor` 拆成 `executor/internalagent` 和部分 `safety/verifier` 能力。
- `libtest` 并入 `verifier`，作为新库验收能力之一。
- `issuewatch`、`todo` 逐步并入 `task`。
- `worker` 被 `orchestrator` 替代。
- `agent` 保留为 TUI 聊天入口，不再承担核心业务编排。
- `github/client.go` 保持工具层，后续拆成 issue、pr、repo、git 等文件。

## 3. Task 类型与状态机

核心对象为 `Task`。不同业务只是不同行为类型。

```text
TaskType:
  issue_to_pr        # Issue → 修复 → 测试 → PR
  issue_reply        # Issue → 分析 → 回复
  pr_fix             # PR → 检测/评论 → 修复 → 更新 PR
  new_repo_accept    # 新库 → 格式检测 → demo/Arduino 测试 → 修复建议或 PR
  knowledge_learn    # 从日志/结果/人工反馈中沉淀知识
```

统一状态机：

```text
created
  → collecting_context
  → planning
  → awaiting_approval
  → executing
  → verifying
  → evaluating_safety
  → awaiting_human_review
  → publishing
  → learning
  → done

任意阶段可进入:
  failed
  cancelled
  rolled_back
  needs_manual_intervention
```

默认自动化边界：

- 允许自动收集上下文、分析 Issue/PR、生成计划、修改代码、补 demo/测试、调用验证命令、创建或更新 PR、写入低风险知识记录。
- 必须人工确认 merge PR、close issue、删除大量文件、force push、rebase 公共分支、破坏性回滚、安全敏感配置修改，以及置信度不足但影响面大的自动回复。

回滚不是简单按钮，而是安全评估结果之一：

```text
验证失败 + diff 小 + 原因明确
  → 继续修复

验证失败 + diff 大 + 多轮修复仍失败
  → 建议回滚，等待人工确认

验证通过 + diff 风险低
  → 创建 PR

验证通过 + diff 风险高
  → 创建 draft PR，并要求人工 review
```

## 4. 执行器与插件机制

执行层抽象为 `Executor`。编排器不关心谁在改代码，只关心输入计划、输出结果、日志、测试结果和 diff。

执行器类型：

```text
internal-agent
  使用当前 Go 实现的 investigator/refactor 能力。
  适合小修、小范围 demo 补全、文档修改、简单 bugfix。

plugin
  调用外部现成能力，例如 Cursor Agent、其他代码修复工具、后续 MCP/插件。
  适合复杂代码修改、跨文件重构、需要更强模型能力的任务。

cli
  调用确定性工具，例如 gofmt、go test、arduino-cli、npm test、repo_validate。
  适合验证、格式化、生成报告。

manual
  人工接管。Agent 只保留上下文、建议和下一步动作。
```

选择规则：

```text
简单 Issue / 小范围修改
  → internal-agent

涉及多文件、架构变更、复杂测试失败
  → plugin

确定性验证或格式化
  → cli

风险高、需求不清、连续失败
  → manual
```

插件不得直接拥有 GitHub 写权限。插件只产出修改后的工作区、diff、执行日志、测试结果和风险声明。真正的 GitHub 写入由 `orchestrator + publisher + github` 统一处理。

## 5. Skill、Knowledge 与 Memory

知识系统拆成三层：

```text
Skill      # 稳定流程：怎么处理某类任务
Knowledge  # 可检索知识：规范、文档、案例、仓库画像
Memory     # 运行记忆：某次任务的日志、结果、评分、失败原因
```

职责：

- Skill 定义任务流程、可用工具、验证要求、风险边界和输出格式。
- Knowledge 存放 Arduino 库规范、repo 格式规范、datasheet、历史成功修复案例、常见报错与解决方案、仓库专属说明。
- Memory 存放每个 Task 的执行计划、日志、diff 摘要、测试结果、用户是否接受、PR 是否合并、后续是否被回滚。

学习流程：

```text
Task 完成或失败
  → 收集事件、日志、diff、测试结果、用户反馈
  → LLM 总结为结构化经验
  → 标记来源和置信度
  → 写入 Memory
  → 高价值经验提升为 Knowledge
  → 必要时建议更新 Skill，但不自动改 Skill
```

知识置信度：

```text
raw        # 原始日志、评论、失败信息，只做证据
summary    # Agent 总结，未人工确认
verified   # 测试通过或 PR 合并验证
approved   # 人工确认过，可强影响后续决策
deprecated # 后续证明错误，不再优先使用
```

RAG 检索应结合过滤条件：

```text
task_type
repo
language
framework
hardware_platform
error_signature
confidence
last_success_time
```

原则：RAG 可以建议，Skill 负责约束，Safety 负责最终拦截。

## 6. 验证、Arduino CLI 与回踩

验证系统独立成 `verifier`。执行器负责修改，验证器负责证明修改是否成立，安全模块负责判断修改是否值得保留。

验证类型：

```text
format
  - gofmt
  - go vet
  - repo_validate
  - library.properties / keywords.txt / README 规范

unit_test
  - go test ./...
  - npm test
  - pytest
  - cargo test

demo_test
  - demo 数量检测
  - 示例代码结构检测
  - README 示例一致性检测

arduino_test
  - arduino-cli core install
  - arduino-cli lib install / 本地库路径
  - arduino-cli compile --fqbn ...
  - 多 board matrix 编译

pr_check
  - GitHub checks
  - merge conflict
  - PR body/title 规范
```

Arduino CLI 配置示例：

```yaml
verifier:
  arduino:
    enabled: true
    cli: arduino-cli
    fqbn:
      - arduino:avr:uno
      - esp32:esp32:esp32
    examples:
      mode: all
    timeout: 120s
```

回踩机制：

- 主动回踩：修改后重新跑验证矩阵，判断是否引入新失败。
- 历史回踩：新 PR 或新知识产生后，找相似历史失败案例重新比对，判断是否踩回老坑。

Safety 输出：

```text
safe_to_publish
publish_as_draft
needs_more_fix
needs_manual_review
recommend_rollback
block
```

回滚策略：

- 非破坏性回滚可以撤销本次 Task 工作区变更、删除临时分支、标记 Task 状态并记录失败经验。
- 破坏性或远端回滚必须人工确认，包括 revert 已推送 commit、force push、关闭 PR、修改已发布内容。

## 7. GitHub 写回策略

GitHub 写回集中放在 `publisher`，executor、plugin、verifier 不直接写 GitHub。

写回动作分级：

```text
低风险，可自动:
  - 添加普通 Issue 评论
  - 添加 PR 检测结果评论
  - push 到 Agent 自己创建的分支
  - 创建 draft PR
  - 更新 Agent 自己创建的 PR body
  - 添加 label，例如 ops-agent:needs-review

中风险，默认可自动但需策略允许:
  - 创建 ready PR
  - 更新已有 PR 分支
  - 请求 review
  - 修改 PR title/body

高风险，必须人工确认:
  - merge PR
  - close issue
  - reopen issue
  - 删除分支
  - force push
  - revert 远端已合并提交
  - 修改 release/tag
```

所有 Agent 写回建议带 footer：

```text
---
Handled by Ops-Agent
Task: <task-id>
Run: <run-id>
Mode: auto | semi | manual
```

GitHub 工具层建议拆分：

```text
internal/github/
  issue.go
  pr.go
  repo.go
  git.go
  client.go
```

业务策略不要写进 `github`。例如“什么情况下创建 draft PR”属于 `publisher/policy`，不是 `github.PRCreate` 的职责。

## 8. 运行形态

系统采用混合模式：

```text
ops-agent tui
  人工界面：查看任务、审批、接管、编辑草稿、管理知识库。

ops-agent daemon
  后台服务：监听 webhook、跑 Task 队列、自动处理低/中风险任务。

ops-agent ci
  CI/headless：执行 PR 检测、repo 验收、Arduino 编译，不维护长期队列。
```

运行模式可通过配置或环境变量选择：

```text
OPS_AGENT_MODE=tui
OPS_AGENT_MODE=daemon
OPS_AGENT_MODE=ci
```

TUI 从 Issue 待办升级为 Task 控制台，负责查看队列、时间线、审批点、diff、日志、测试结果、知识库和插件状态。

Daemon 负责 webhook、Task 队列、编排、通知和学习。

CI 只做一次性验证，输出明确 exit code，不维护长期队列。

## 9. 迁移路线

阶段 1：建立新核心骨架。

```text
internal/task
internal/orchestrator
internal/executor
internal/verifier
internal/safety
internal/knowledge
internal/memory
internal/publisher
internal/skill
```

阶段 2：迁移 Issue → PR 闭环。

```text
Issue webhook / TUI 指令
  → issue_to_pr Task
  → 收集上下文
  → internal-agent 修改代码
  → verifier 跑测试
  → safety 判断
  → publisher 创建 draft PR
  → knowledge 记录经验
```

阶段 3：加入插件执行器。

阶段 4：强化验证与 Arduino CLI。

阶段 5：知识库自动学习。

阶段 6：完善 TUI 控制台。

第一轮实现范围：

```text
1. Task 核心模型
2. issue_to_pr Task
3. internal-agent 执行器适配旧 refactor
4. verifier 跑 go test / repo_validate
5. safety 基础 diff 风险判断
6. publisher 创建 draft PR
7. memory 记录任务结果
```

暂缓范围：

- 完整插件系统。
- 完整 Arduino CLI matrix。
- 自动更新 Skill。
- 全自动 merge / close issue。
- 高级 TUI 布局。

## 10. 设计自审

- 无占位要求：本文中“暂缓范围”均为明确后续阶段，不作为当前设计缺口。
- 架构一致性：Task 是唯一业务编排核心，executor、verifier、safety、publisher 各自收口。
- 范围控制：第一轮只实现 `issue_to_pr` 闭环，避免一次性实现所有自动化场景。
- 风险边界：GitHub 高风险写操作、破坏性回滚、merge/close 等动作均需要人工确认。
- 知识边界：RAG 存事实和经验，Skill 存流程和护栏，Safety 做最终拦截。

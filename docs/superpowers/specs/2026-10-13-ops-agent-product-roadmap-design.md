# Ops-Agent 产品路线图

> 日期：2026-10-13
> 状态：已确认。v1 实现计划见 `docs/superpowers/plans/2026-10-13-ops-agent-v1-scope.md`。v2、v3 不在该计划内。
> 范围：当前产品目标、三阶段完成标准、明确不做，以及 v1 仓库里允许存在的代码。本文件不改已有文档正文。v1 实现时要删掉非 v1 代码。

## 1. 产品目标

Ops-Agent 是一个自动处理 GitHub 工作的 Agent。总目标有四件事：

1. 处理 Issue 和 PR。
2. 检查并修复库的规范、demo 和编译问题。
3. 从处理结果里学习，提高后续同类任务的效率。
4. 让人能看到任务日志和状态。

这四件事按顺序交付。前一阶段没有达到完成标准，不开始下一阶段的功能。

## 2. 阶段顺序

### v1 Issue / PR 闭环

TUI 开着时，扫描或 webhook 命中的 Issue、PR 自动入队。模型判断类型。问答只在原 Issue 或来源 PR 上评论。库缺陷改代码，Arduino CLI 识别库并编译全部 `examples/` 后评论并开 draft PR。人在现有 TUI 里看任务列表、阶段、日志和失败原因。合并、关闭 Issue、force push 由人来做。细则见 `docs/superpowers/specs/2026-10-14-ops-agent-library-fix-pr-design.md`。

### v2 库验收与修复

对新库或已有库做规范、demo、编译检查，并修改能修的错误。沿用 v1 的任务、日志和 draft PR 边界。

### v3 自动学习

用已完成任务的结果提高后续处理效率。学习不改变安全边界，也不能代替人做合并、关闭 Issue 或 force push。

## 3. v1 完成标准

v1 只交付一条闭环。TUI 处于打开状态时，扫描或 webhook 命中的 Issue、PR 自动进入队列，不需要人逐条启动。两条入口都算命中：仓库扫描，或 TUI 进程收到的 webhook。

一条任务自动走到这些结果：

- 读入该 Issue 或 PR 的标题、正文和最近用户评论。
- 克隆任务所属仓库到临时目录。不使用 TUI 当前目录冒充目标仓库。
- 无文件改动：只发一条评论，不开 PR。
- 有文件改动：Arduino CLI 通过且非高风险后，原 Issue 有评论且存在 draft PR。
- 同一条 Issue 或 PR 的重复事件复用已有任务。已完成的不再发第二条评论、不再开第二个 PR。

人在现有 TUI 里能看到：任务列表、当前阶段、日志、失败原因、评论是否已发出、draft PR 地址。出现下面任一情况时，任务停在可查看状态，并写明原因。失败后不发评论、不开 PR（评论已发出后的发布失败除外）。

- 读取 Issue / PR、克隆仓库或发表评论失败。
- 缺 arduino-cli、库无法识别、示例编译失败、缺少示例、高风险改动。
- 超过任务时限。模型没有自己产出评论时不发评论。

v1 里人必须亲自做的动作：合并 PR、关闭 Issue、force push。

v1 完成的验收是：TUI 打开期间，问答 Issue 出现一条 Agent 评论且无 PR；库缺陷 Issue 在 Arduino 通过后出现评论和 draft PR；失败时 TUI 能看到停在哪一步、为什么停。完整库规范/demo 验收、自动学习、Web 后台、关掉 TUI 后继续跑的常驻进程，都不算 v1 完成条件。

## 4. v2 完成标准

v2 只在 v1 验收通过之后开始。它处理新库和已有库，自动做三类检查：仓库规范、demo、编译。能修的错误由 Agent 改完，并再跑同一组检查。检查通过后，仍推到 Agent 自己的新分支并开 draft PR。失败、属于第 3 节定义的高风险，或修不好时停住，并在 TUI 里写明停在哪一项检查、为什么停。合并、关闭 Issue、force push 仍由人来做。

v2 完成的验收是：一条命中的库任务能自动跑完规范、demo、编译检查；能修的问题变成 draft PR；修不好的问题在 TUI 里看得到原因。

## 5. v3 完成标准

v3 只在 v2 验收通过之后开始。它使用已经结束的任务结果，包括成功的 draft PR、失败原因和人最终是否接受，让后续同类任务少走重复弯路。学习结果要能在 TUI 里看到它影响了哪次任务、用了哪条经验。学习不能放宽安全边界，也不能代替人做合并、关闭 Issue 或 force push。

v3 完成的验收是：一次新任务能用到此前已结束任务的结果，并且 TUI 能看到用了哪条经验。

## 6. 明确不做

所有阶段里，Agent 都不自动做这三件事：合并 PR、关闭 Issue、force push。

v1 期间不做这些：

- 库的规范、demo、编译检查和自动修复。
- 从历史任务自动学习，并用学习结果改变后续处理。
- 单独的 Web 后台。
- 关掉 TUI 之后仍继续接事件、跑任务的常驻进程。
- 为了架构完整先补齐插件系统、成本阻断、Arduino 多板矩阵、自动改流程规则。

v2 开始前不把库验收算进完成标准。v3 开始前不把自动学习算进完成标准。没到对应阶段，不把该阶段的代码留在仓库里。

## 7. 文档效力

本路线图是当前的产品目标。以后判断“现在要做什么”，以这里的阶段顺序、完成标准和明确不做为准。

下列文档保留，当作已有方案的记录。它们和本路线图冲突时，当前目标以本路线图为准：

- `README.md`
- `docs/DESIGN.md`
- `docs/ROADMAP.md`
- `docs/superpowers/specs/2026-10-13-ops-agent-task-orchestrator-design.md`

本路线图文件本身不修改上述文档的正文。v1 实现必须按第 9 节删除非 v1 代码。

## 8. v1 有效流程

v1 的有效流程只有这一条：

1. TUI 打开。
2. 扫描或 webhook 命中 Issue 或 PR。
3. 自动入队。
4. 读上下文并克隆目标仓库。
5. 执行器调查，问答不改文件，缺陷可 edit_file，最后 reply。
6. 无改动则评论。有改动则 Arduino CLI，通过后评论并开 draft PR。
7. 克隆失败、Arduino 失败、高风险、评论/发布失败或超过时限时停住，并在 TUI 显示原因。

合并、关闭 Issue、force push 由人来做。

## 9. v1 代码范围

v1 的仓库只保留这条闭环用得到的代码。检查 v1 时，以主工作区为准：非 v1 包必须不存在，留下的包里也不能带第 6 节列出的行为。v2、v3 的代码到该阶段开始时再加回来。`.worktrees/` 不作为第二份现行代码；v1 检查前把它移出主工作区。

下面是允许留下的上限。某个包如果 v1 闭环没有调用，也删除。

v1 允许留下的包：

- `cmd/ops-agent`
- `internal/tui`：任务列表、阶段、日志、失败原因，以及 TUI 打开期间的入队
- `internal/config`：这条闭环要读的配置
- `internal/github`
- `internal/ai`
- `internal/netproxy`：仅当模型或 GitHub 请求需要代理
- `internal/webhook`、`internal/smee`：TUI 打开期间接收 webhook
- `internal/issuecomment`：读 Issue 上下文时过滤评论
- `internal/task`
- `internal/orchestrator`
- `internal/workspace`
- `internal/executor`
- `internal/verifier`：只跑目标仓库已有测试
- `internal/safety`：只实现第 3 节的高风险判断
- `internal/publisher`：推送 Agent 自己的分支、创建 draft PR、评论该 PR
- `internal/memory`：供 TUI 显示的任务日志

v1 整包删除，到后续阶段需要时再加：

- `internal/libtest`、`internal/repovalidate`：库验收，属于 v2
- `internal/knowledge`、`internal/skill`：自动学习与流程规则，属于 v3
- `internal/budget`、`internal/feedback`：成本阻断和外部反馈分类；v1 的连续 3 次失败停住写在编排流程里
- `internal/notify`、`internal/prcheck`、`internal/headless`：通知、CI 门禁、关掉 TUI 后的另一套入口

留下的包如果还包含库验收、学习、通知、CI 门禁、Arduino、插件或常驻进程，这些行为一并删除。v1 完成时，主工作区里搜索不到这些被删除包的导入。

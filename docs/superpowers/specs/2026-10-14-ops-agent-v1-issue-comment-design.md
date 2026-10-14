# Ops-Agent v1 改为 Issue 评论闭环

> 日期：2026-10-14
> 状态：已确认。实现计划见 `docs/superpowers/plans/2026-10-14-ops-agent-v1-issue-comment.md`。
> 范围：把 v1 自动结果从 draft PR 改成原 Issue / 来源 PR 上的评论，并修掉当前任务跑不完的原因。
> 依据：已确认的产品选择（评论代替 draft PR；不强制评论；直到模型自己 reply 或出错或超时）。
> 本文件与 `docs/superpowers/specs/2026-10-13-ops-agent-product-roadmap-design.md` 第 2、3、8 节中“开 draft PR”的 v1 完成标准冲突时，以本文件为准。确认后实现时同步改路线图这三节。不改 `README.md`、`docs/DESIGN.md`、`docs/ROADMAP.md`、`docs/superpowers/specs/2026-10-13-ops-agent-task-orchestrator-design.md` 的正文。

## 1. 目标

自动任务的完成结果是：在来源 Issue 或来源 PR 上发出一条 Agent 评论。不改代码，不开 draft PR，不推分支。

人仍只在对话确认后做合并、关闭 Issue、force push。

## 2. 有效流程

1. TUI 打开。扫描或 webhook 命中 Issue 或 PR，自动入队。
2. 用 `gh` 读取该对象的标题、正文、最近用户评论（排除 Agent 自己的评论）。
3. 把 Issue / PR 所属仓库克隆到临时目录。来源仓库以任务的 `repo` 字段为准，例如 `tangjie133/test`。禁止用 TUI 进程的当前工作目录充当目标仓库。
4. 执行器在该副本里只读调查。
5. 模型输出 `{"action":"reply","body":"..."}` 后，把 `body` 发到原 Issue 或来源 PR。来源是 PR 时，评论发在该 PR 上（GitHub 把 PR 当 Issue 评论）。
6. 评论成功则任务完成。TUI 展开行里原来的「PR」改为「评论」，已发出时写「已发出」，未发出时写「—」。

同一条来源的重复事件复用任务。状态已是完成时不再发第二条评论。对话「重试」把同一条任务重新设为可执行，允许再评一次。

## 3. 执行器

可用 action 只有：

- `list_dir`
- `search_repo`
- `read_file`
- `reply`（唯一结束动作）

删除执行器里的 `edit_file` 和 `done`。模型输出其它 action 时，把「不支持」写回观察结果，继续下一轮，不结束任务。

JSON 解析失败时，要求模型只输出一个 JSON action，继续下一轮，不结束任务，不发评论。

循环结束条件只有这些：

- 模型输出合法 `reply` 且评论发送成功。
- 读取 Issue / PR 失败、克隆失败、模型 HTTP 调用失败、评论发送失败。任务停住，`StopReason` 写成错误原文。
- 超过 `task.budget.max_duration`（默认 30 分钟）。任务停住，`StopReason` 写成「超过任务时限」，不发评论。

`task.budget.max_steps` 不再用来结束循环。没有步数用尽就强制评论这件事。

## 4. 工作区

每次运行在临时目录里 `gh repo clone` 任务的 `repo`。只读，不建 `ops-agent/issue-N` 分支。成功后删除临时目录。失败且配置 `keep_failed` 为真时保留，便于查看。

## 5. 自动队列不再做的事

- 改文件
- 跑仓库测试
- 推送 Agent 分支
- 创建 draft PR
- 在步数用尽时拼一条评论发出去
- 合并、关闭 Issue、force push

`internal/verifier` 和 `internal/safety` 本轮闭环不再调用。本规格不要求立刻删这两个包。

`internal/publisher` 的自动路径只保留 `CommentIssue`。`CreateDraftPR` 和 `CommentDraftPR` 本轮不调用。

## 6. 失败与界面

失败必须写入任务的 `StopReason`，TUI 左侧展开的「原因」显示该字段，缺省为「—」。记忆日志仍追加同一句原因。

成功时 `StopReason` 为空，评论行显示「已发出」。

## 7. 验收

TUI 打开期间，命中 `tangjie133/test#51` 这类问答 Issue 时：工作区是 `tangjie133/test` 而不是 Ops-Agent 仓库；模型能读到「简单介绍库功能」和用户评论；模型自己 `reply` 后原 Issue 出现一条带 Agent 页脚的评论；任务状态为完成。克隆失败或超时时不发评论，TUI 能看到原因。

# Ops-Agent 库问题：Arduino 测试后评论并开 draft PR

> 日期：2026-10-14
> 状态：已确认
> 范围：区分问答 Issue 和库本身的代码问题。后者改代码、本地 Arduino CLI 测试通过后再评论并开 draft PR。
> 依据：已确认的选择（改代码 → Arduino CLI → 评论 → draft PR；全部示例编译 + 库可被识别；模型判断类型）。
> 与 `docs/superpowers/specs/2026-10-14-ops-agent-v1-issue-comment-design.md` 冲突时，以本文件为准。确认后实现时同步改路线图第 2、3、8 节。不改 `README.md`、`docs/DESIGN.md`、`docs/ROADMAP.md`、`docs/superpowers/specs/2026-10-13-ops-agent-task-orchestrator-design.md` 的正文。

## 1. 目标

自动任务仍由模型判断 Issue 类型。

- 问答（例如「介绍库」）：只在原 Issue 或来源 PR 上评论。不改文件，不开 PR。
- 库本身问题（例如 `read` 忽略 `size` 导致溢出）：改最小相关代码，用 Arduino CLI 在本地测过，再评论 Issue，并开 draft PR。

合并、关闭 Issue、force push 仍由人在对话里确认后执行。

## 2. 有效流程

1. TUI 打开，扫描或 webhook 入队。
2. 读取 Issue / PR 的标题、正文、最近用户评论（排除 Agent 评论）。
3. 按任务的 `repo` 克隆到临时目录，不用 TUI 当前目录。
4. 执行器在该副本里调查，必要时改文件，最后 `reply`。
5. 编排器看工作区有没有改动：
   - 无改动：把 `reply` 发到原 Issue 或来源 PR，任务完成。不开 PR。
   - 有改动：先跑第 4 节 Arduino CLI。通过则评论，再推 Agent 分支并开 draft PR。失败则停住，不评论、不开 PR。
6. 高风险改动（删除文件，或路径含 CI 工作流、发布配置、权限、密钥）即使编译通过也停住，不评论、不开 PR。TUI 写明原因。
7. 同一来源重复 webhook 复用任务。已完成的不再自动再评、不再开第二个 PR。对话「重试」才再跑。

## 3. 执行器

可用 action：

- `list_dir`
- `search_repo`
- `read_file`
- `edit_file`（已有文件必须 old/new；仅新建文件可用 content）
- `reply`（结束动作，body 是 Issue 评论）

模型在调查后必须 `reply`。系统提示写明：问答不要改文件；库的缺陷要改最小片段，不要跑测试命令（测试由编排器在结束后跑）。

循环仍跟调用方 `ctx`（任务时限）。不因单次空 `content` 整任务失败：读 `reasoning_content`；仍空则要求再输出 JSON。

## 4. Arduino CLI

有文件改动时，编排器在工作区执行，不 upload。

通过条件三项都要满足：

1. 本机有 `arduino-cli`。没有则停住，原因「缺少 arduino-cli」。
2. 该工作区作为库能被识别：用 `arduino-cli compile --library <工作区>` 编译示例；识别失败则停住并写 CLI 原文。
3. 编译 `examples/` 下全部 `.ino` 示例（含子目录）。任一失败则停住，原因含失败的示例路径和 CLI 原文。没有 `examples/` 或没有 `.ino` 则停住，原因「缺少验证」。

FQBN：

- `library.properties` 的 `architectures` 含 `esp32`：`esp32:esp32:esp32`
- 含 `avr`：`arduino:avr:uno`
- `*`、空、或其它未列出的值：`arduino:avr:uno`

不在执行器里跑 CLI。测试未通过时不把「未测试」当成通过。

## 5. 发布

Arduino 通过且非高风险：

1. 在 Agent 分支 `ops-agent/issue-N` 或 `ops-agent/pr-N` 提交并推送（只推自己的分支）。
2. `CommentIssue` 把执行器的 `reply` 发到原 Issue 或来源 PR。
3. `CreateDraftPR`，标题用 Issue 标题，head 为该分支，base 为仓库默认分支（没有则 `main`）。
4. TUI：评论行「已发出」；PR 行显示 draft PR 地址。

问答路径：只有评论行「已发出」，PR 行「—」。

## 6. 失败

写入 `StopReason`，左侧「原因」显示。不发评论、不开 PR：

- 克隆、读 Issue、模型 HTTP、任务时限
- 缺少 arduino-cli、库无法识别、示例编译失败、缺少示例
- 高风险路径或删除文件
- 推送或创建 draft PR 失败（若评论已发出，原因写发布失败，任务仍算失败，不自动再评）

Agent 自己的评论不重新入队。

## 7. 验收

- `tangjie133/test#51` 这类问答：只评论，无 PR。
- `tangjie133/test2#2` 这类库缺陷：有代码改动，Arduino CLI 通过后 Issue 有评论且存在 draft PR。
- 编译失败：状态失败，原因含 arduino-cli 输出，GitHub 上无新评论、无新 PR。

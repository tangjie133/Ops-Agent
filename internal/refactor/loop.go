package refactor

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/investigator"
)

const systemPrompt = `你是 Ops-Agent 代码重构助手。用户已确认修库，你需要在已克隆仓库的功能分支上实现修复，并通过测试与 repo_validate。

## 工作方式
每一轮你必须且只能输出一个 JSON 对象（不要 markdown 代码块，不要其它文字）。

可用 action：
1. {"action":"search_repo","query":"关键词"} — grep 源码
2. {"action":"read_file","path":"相对路径","start_line":1,"end_line":200}
3. {"action":"list_dir","path":""} — 列目录
4. {"action":"fetch_url","url":"https://..."}
5. {"action":"web_search","query":"..."}
6. {"action":"rag_search","query":"..."}
7. {"action":"repo_validate","query":"规范名或空"}
8. edit_file — 改代码（两种方式，二选一）：
   - 修改已有文件（推荐）：{"action":"edit_file","path":"相对路径","old":"与 read_file 完全一致的原文片段","new":"替换后的片段"}
   - 仅新建文件：{"action":"edit_file","path":"相对路径","content":"完整新文件内容"}
   禁止对已有文件使用 content 整文件覆盖；禁止删除未涉及的函数、类型或逻辑。
9. {"action":"run_cmd","command":"go test ./..."} — 白名单：go test/build/vet、make、npm/yarn test、cargo test/build、pytest、git status/diff
10. {"action":"done","body":"变更摘要"} — 修改完成并测试通过后结束

## 策略
- 先读 Issue 与已有分析草稿，再 search/read 定位问题
- 只改与 Issue 相关的最小代码：用 old/new 替换单个函数、分支或几行，保留其余代码不动
- 改前必须 read_file 目标区域；old 须从 read_file 结果原样复制（含空格与换行）
- 小步修改，每次 edit_file 后 run_cmd 或 git diff 验证
- 必须 repo_validate（若配置了规范）
- 不要修改 .git/ 或 knowledge/ 目录
- 完成后 done，body 简述改了什么

收到 tool 结果后继续输出下一个 JSON action，直到 done。`

const forceDonePrompt = `已达到最大步数。不要再调用工具。
直接输出：{"action":"done","body":"..."}`

const parseErrorPrompt = `上一条输出不是合法 JSON action。请只输出一个 JSON 对象，例如：
{"action":"read_file","path":"src/main.go","start_line":1,"end_line":80}`

// Loop 多轮重构 Agent 主循环。
type Loop struct {
	cfg    config.InvestigatorConfig
	client investigator.LLM
	tools  *Toolbox
	log    investigator.Logger
}

func NewLoop(cfg config.InvestigatorConfig, client investigator.LLM, tools *Toolbox) *Loop {
	cfg.Normalize()
	return &Loop{cfg: cfg, client: client, tools: tools}
}

func (l *Loop) SetLogger(log investigator.Logger) {
	l.log = log
}

func (l *Loop) Run(ctx context.Context, issuePrompt string) (string, error) {
	msgs := []investigator.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: issuePrompt},
	}

	toolErrors := 0
	for step := 1; step <= l.maxSteps(); step++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		raw, err := l.client.ChatMessages(ctx, msgs)
		if err != nil {
			return "", err
		}

		action, err := ParseAction(raw)
		if err != nil {
			toolErrors++
			msgs = append(msgs,
				investigator.Message{Role: "assistant", Content: raw},
				investigator.Message{Role: "user", Content: parseErrorPrompt},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				return l.forceDone(ctx, msgs)
			}
			continue
		}

		if action.Action == ActionDone {
			if err := action.Validate(); err != nil {
				return "", err
			}
			return strings.TrimSpace(action.Body), nil
		}

		if err := action.Validate(); err != nil {
			toolErrors++
			msgs = append(msgs,
				investigator.Message{Role: "assistant", Content: raw},
				investigator.Message{Role: "user", Content: "tool error: " + err.Error() + "\n" + parseErrorPrompt},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				return l.forceDone(ctx, msgs)
			}
			continue
		}

		obs, err := l.tools.Run(ctx, action)
		if err != nil {
			toolErrors++
			msgs = append(msgs,
				investigator.Message{Role: "assistant", Content: raw},
				investigator.Message{Role: "user", Content: fmt.Sprintf("tool error: %v\n%s", err, parseErrorPrompt)},
			)
			if toolErrors >= l.cfg.MaxToolErrors {
				return l.forceDone(ctx, msgs)
			}
			continue
		}

		toolErrors = 0
		toolMsg := fmt.Sprintf("tool result (%s):\n%s", action.Action, obs)
		msgs = append(msgs,
			investigator.Message{Role: "assistant", Content: raw},
			investigator.Message{Role: "user", Content: toolMsg},
		)

		if step == l.maxSteps() {
			return l.forceDone(ctx, msgs)
		}
	}

	return "", fmt.Errorf("refactor: exhausted steps without done")
}

func (l *Loop) maxSteps() int {
	if l.cfg.MaxSteps <= 0 {
		return 12
	}
	return l.cfg.MaxSteps
}

func (l *Loop) forceDone(ctx context.Context, msgs []investigator.Message) (string, error) {
	msgs = append(msgs, investigator.Message{Role: "user", Content: forceDonePrompt})
	raw, err := l.client.ChatMessages(ctx, msgs)
	if err != nil {
		return "", err
	}
	action, err := ParseAction(raw)
	if err != nil {
		return strings.TrimSpace(raw), nil
	}
	if action.Action == ActionDone && strings.TrimSpace(action.Body) != "" {
		return strings.TrimSpace(action.Body), nil
	}
	return strings.TrimSpace(raw), nil
}

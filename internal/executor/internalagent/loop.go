package internalagent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/ai"
	"github.com/ZzedJay/Ops-Agent/internal/executor"
)

const systemPrompt = `你是 Ops-Agent 执行器。每一轮只输出一个 JSON 对象，不要 markdown，不要其它文字。

可用 action：
1. {"action":"list_dir","path":""}
2. {"action":"search_repo","query":"关键词"}
3. {"action":"read_file","path":"相对路径","start_line":1,"end_line":200}
4. {"action":"edit_file","path":"相对路径","old":"原文片段","new":"替换片段"}  // 已有文件必须 old/new
5. {"action":"edit_file","path":"相对路径","content":"完整内容"}  // 仅当文件尚不存在
6. {"action":"reply","body":"Issue 评论正文"}

问答 Issue 不要改文件。库的缺陷只改最小相关片段。不要运行测试或 arduino-cli。不要声称已经合并或关闭 Issue。完成后必须 reply。`

var jsonBlockRE = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

type action struct {
	Action    string `json:"action"`
	Query     string `json:"query,omitempty"`
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Body      string `json:"body,omitempty"`
	Old       string `json:"old,omitempty"`
	New       string `json:"new,omitempty"`
	Content   string `json:"content,omitempty"`
}

func runLoop(ctx context.Context, llm LLM, req executor.Request) (string, error) {
	tools := newToolbox(req.WorkDir)
	msgs := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: issuePrompt(req)},
	}
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		raw, err := llm.ChatMessages(ctx, msgs)
		if err != nil {
			if ctx.Err() != nil {
				return "", err
			}
			if strings.Contains(err.Error(), "empty content") || strings.Contains(err.Error(), "empty model output") {
				msgs = append(msgs, ai.Message{Role: "user", Content: "没有可用输出，只输出一个 JSON action。"})
				continue
			}
			return "", err
		}
		act, err := parseAction(raw)
		if err != nil {
			msgs = append(msgs, ai.Message{Role: "assistant", Content: raw}, ai.Message{Role: "user", Content: "只输出一个 JSON action。"})
			continue
		}
		if act.Action == "reply" {
			body := strings.TrimSpace(act.Body)
			if body == "" {
				body = "已完成调查"
			}
			return body, nil
		}
		obs, err := tools.run(ctx, act)
		if err != nil {
			obs = "错误: " + err.Error()
		}
		msgs = append(msgs, ai.Message{Role: "assistant", Content: raw}, ai.Message{Role: "user", Content: obs})
	}
}

func parseAction(raw string) (action, error) {
	raw = strings.TrimSpace(raw)
	candidates := []string{raw}
	if m := jsonBlockRE.FindStringSubmatch(raw); len(m) > 1 {
		candidates = append([]string{m[1]}, candidates...)
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			candidates = append(candidates, raw[i:j+1])
		}
	}
	var last error
	for _, c := range candidates {
		var a action
		if err := json.Unmarshal([]byte(c), &a); err != nil {
			last = err
			continue
		}
		a.Action = strings.ToLower(strings.TrimSpace(a.Action))
		if a.Action == "" {
			last = fmt.Errorf("missing action")
			continue
		}
		return a, nil
	}
	if last == nil {
		last = fmt.Errorf("empty model output")
	}
	return action{}, last
}

package task

import (
	"fmt"
	"strings"
)

// MakeTaskKey 生成稳定任务键：同一仓库、类型和来源编号复用同一 Task。
func MakeTaskKey(repo string, typ Type, kind SourceKind, number int) string {
	repo = strings.ToLower(strings.TrimSpace(repo))
	return fmt.Sprintf("%s|%s|%s|%d", repo, typ, kind, number)
}

// MakeRunKey 生成单次执行键：同一 Task 的同一触发内容不重复跑。
func MakeRunKey(taskKey, triggerHash string) string {
	return taskKey + "|" + strings.TrimSpace(triggerHash)
}

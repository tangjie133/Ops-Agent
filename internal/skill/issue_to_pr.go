// Package skill 存放稳定工作流护栏。第一轮只提供 issue_to_pr 文本约束。
package skill

// Name 是 issue_to_pr 工作流名称。
const Name = "issue-to-pr"

// Directive 返回执行器必须遵守的中文护栏。
func Directive() string {
	return "只修改与 Issue 相关的文件。修改后必须运行验证。只创建 draft PR，不合并、不关闭 Issue。"
}

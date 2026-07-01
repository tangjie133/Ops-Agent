// Package investigator 实现多轮 Issue 调查 Agent。
//
// 通过 LLM 与工具箱（读文件、grep、fetch URL、RAG 检索等）循环推理，
// 最终生成 Issue 回复草稿或结构化动作。
package investigator

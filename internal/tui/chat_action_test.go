package tui

import (
	"strings"
	"testing"
)

func TestHighRiskNeedsConfirm(t *testing.T) {
	next, reply := HandleChat(nil, "合并 o/r#9")
	if reply.Execute != nil || next == nil || next.Kind != "merge" || next.Number != 9 {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
	if !strings.Contains(reply.Text, "o/r") || !strings.Contains(reply.Text, "9") {
		t.Fatalf("text=%s", reply.Text)
	}
	next, reply = HandleChat(next, " 确认 ")
	if next != nil || reply.Execute == nil || reply.Execute.Kind != "merge" {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestOtherTextCancelsConfirm(t *testing.T) {
	pending := &ChatAction{Kind: "close_issue", Repo: "o/r", Number: 4}
	next, reply := HandleChat(pending, "先别关")
	if next != nil || reply.Execute != nil || !strings.Contains(reply.Text, "不执行") {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestNewHighRiskReplacesPending(t *testing.T) {
	pending := &ChatAction{Kind: "merge", Repo: "o/r", Number: 1}
	next, reply := HandleChat(pending, "force push o/r 分支 ops-agent/issue-2")
	if next == nil || next.Kind != "force_push" || !strings.Contains(reply.Text, "已取消") {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestStopExecutesWithoutSecondConfirm(t *testing.T) {
	next, reply := HandleChat(nil, "停止 o/r#46")
	if next != nil || reply.Execute == nil || reply.Execute.Kind != "stop" || reply.Execute.Number != 46 {
		t.Fatalf("next=%+v reply=%+v", next, reply)
	}
}

func TestQuestionGoesToModel(t *testing.T) {
	_, reply := HandleChat(nil, "你好")
	if !chatNeedsModel(reply) {
		t.Fatalf("greeting should ask the model, got %+v", reply)
	}
	_, reply = HandleChat(nil, "停止 o/r#1")
	if chatNeedsModel(reply) {
		t.Fatal("a command must not ask the model")
	}
}

func TestEmptyAndCodeChangeDoNothing(t *testing.T) {
	if _, reply := HandleChat(nil, "   "); reply.Execute != nil || reply.Text != "" {
		t.Fatalf("%+v", reply)
	}
	_, reply := HandleChat(nil, "帮我改代码并开 PR")
	if reply.Execute != nil || !strings.Contains(reply.Text, "任务队列") {
		t.Fatalf("%+v", reply)
	}
}

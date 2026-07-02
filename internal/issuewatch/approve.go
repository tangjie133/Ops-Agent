package issuewatch

import (
	"fmt"

	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

// ConfirmFixResult 确认修库（fix_confirmed）结果。
type ConfirmFixResult struct {
	Confirmed bool
	Item      todo.Item
	Reason    string
}

// ConfirmFixPR 将待办标记为 fix_confirmed，等待 Refactor Worker。
func ConfirmFixPR(store *todo.FileStore, repo string, num int) (*ConfirmFixResult, error) {
	if repo == "" || num <= 0 {
		return nil, fmt.Errorf("invalid issue ref")
	}
	it, ok := store.Get(repo, num)
	if !ok {
		return &ConfirmFixResult{Reason: "not in todo"}, nil
	}
	switch it.Status {
	case todo.StatusFixConfirmed, todo.StatusRefactoring, todo.StatusPROpened:
		return &ConfirmFixResult{Reason: "already confirmed"}, nil
	}
	if !todo.CanConfirmFix(it.Status) {
		return &ConfirmFixResult{Reason: fmt.Sprintf("status %s cannot confirm fix", it.Status)}, nil
	}
	if err := store.Transition(repo, num, todo.StatusFixConfirmed); err != nil {
		return nil, err
	}
	it.Status = todo.StatusFixConfirmed
	return &ConfirmFixResult{Confirmed: true, Item: it}, nil
}

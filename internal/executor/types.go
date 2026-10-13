package executor

import "context"

// Request 是一次代码修改请求。
type Request struct {
	WorkDir string
	Repo    string
	Number  int
	Title   string
	Body    string
}

// Result 是执行器产出的摘要。
type Result struct {
	Summary string
	Changed bool
}

// Executor 在隔离工作区里修改代码。
type Executor interface {
	Run(ctx context.Context, req Request) (Result, error)
}

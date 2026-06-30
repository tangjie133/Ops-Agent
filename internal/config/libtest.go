package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// LibTestConfig 新库验收：格式规范 + examples/demo 合理性。
type LibTestConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Standard       string `yaml:"standard"`        // standards/*.yaml 名称
	MinDemos       int    `yaml:"min_demos"`
	DemoDir        string `yaml:"demo_dir"`
	WorkspaceDir   string `yaml:"workspace_dir"`   // 克隆检出目录，空=默认
	AutoRun        bool   `yaml:"auto_run"`        // true=自动验收 false=手动（Enter）
	OnPush         bool   `yaml:"on_push"`         // push 到默认分支
	OnRelease      bool   `yaml:"on_release"`      // release published
	OnRepoCreated  bool   `yaml:"on_repo_created"` // 新建仓库
	MaxQueueItems  int    `yaml:"max_queue_items"`
}

// RunModeLabel 执行方式（TUI / 摘要）。
func (c LibTestConfig) RunModeLabel() string {
	if c.AutoRun {
		return "自动"
	}
	return "手动"
}

// EnabledLabel 启用状态。
func (c LibTestConfig) EnabledLabel() string {
	if c.Enabled {
		return "已启用"
	}
	return "已禁用"
}

// Summary 一行摘要。
func (c LibTestConfig) Summary() string {
	if !c.Enabled {
		return "已禁用"
	}
	return c.RunModeLabel() + " · " + c.Standard
}

func (c *LibTestConfig) Normalize() {
	if c.Standard == "" {
		c.Standard = "arduino-library"
	}
	if c.MinDemos <= 0 {
		c.MinDemos = 1
	}
	if c.DemoDir == "" {
		c.DemoDir = "examples"
	}
	if c.MaxQueueItems <= 0 {
		c.MaxQueueItems = 30
	}
	if !c.OnPush && !c.OnRelease && !c.OnRepoCreated {
		c.OnPush = true
		c.OnRelease = true
	}
}

func (c LibTestConfig) WorkspacesRoot() string {
	if c.WorkspaceDir != "" {
		return c.WorkspaceDir
	}
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "lib-test", "workspaces")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "lib-test", "workspaces")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "lib-test", "workspaces")
	}
	return filepath.Join("lib-test", "workspaces")
}

func LibTestStorePath() string {
	if p := os.Getenv("OPS_AGENT_DATA"); p != "" {
		return filepath.Join(p, "lib-test", "queue.json")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ops-agent", "lib-test", "queue.json")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ops-agent", "lib-test", "queue.json")
	}
	return filepath.Join("lib-test", "queue.json")
}

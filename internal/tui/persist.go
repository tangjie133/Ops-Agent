package tui

// persist.go — 菜单修改后调用 config.Save 持久化。

import "github.com/ZzedJay/Ops-Agent/internal/config"

// persistConfig 将当前配置写入磁盘。
func persistConfig(cfg *config.Config) string {
	path, err := config.Save(cfg)
	if err != nil {
		return "保存失败: " + err.Error()
	}
	return "已保存至 " + path
}

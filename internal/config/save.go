package config

// save.go — 配置读写路径与 YAML 持久化。

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var loadedConfigPath string

func ConfigPath() string {
	if loadedConfigPath != "" {
		return loadedConfigPath
	}
	return SavePath()
}

func SavePath() string {
	if p := os.Getenv("OPS_AGENT_CONFIG"); p != "" {
		return p
	}
	local := ".ops-agent.yaml"
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return local
}

// Save 将配置写入 SavePath() 指向的 YAML 文件。
func Save(cfg *Config) (string, error) {
	path := SavePath()
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir config dir: %w", err)
		}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write config %s: %w", path, err)
	}
	loadedConfigPath = path
	return path, nil
}

func (w *WebhookConfig) LocalURL() string {
	listen := w.Listen
	if listen == "" {
		listen = "127.0.0.1:8765"
	}
	path := w.Path
	if path == "" {
		path = "/webhooks/github"
	}
	if path[0] != '/' {
		path = "/" + path
	}
	return "http://" + listen + path
}

func (w *WebhookConfig) HealthURL() string {
	listen := w.Listen
	if listen == "" {
		listen = "127.0.0.1:8765"
	}
	return "http://" + listen + "/healthz"
}

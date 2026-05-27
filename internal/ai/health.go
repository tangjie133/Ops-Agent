package ai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
)

type Health struct {
	Reachable bool
	Message   string
}

func CheckHealth(ctx context.Context, cfg config.AIConfig) Health {
	if cfg.BaseURL == "" {
		return Health{Reachable: false, Message: "ai.base_url not configured"}
	}

	url := cfg.BaseURL
	if len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	url += "/models"

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return Health{Reachable: false, Message: err.Error()}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Health{
			Reachable: false,
			Message:   fmt.Sprintf("cannot reach llama-server at %s", cfg.BaseURL),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Health{Reachable: true, Message: fmt.Sprintf("ok (%s)", cfg.Model)}
	}
	return Health{
		Reachable: false,
		Message:   fmt.Sprintf("llama-server returned %s", resp.Status),
	}
}

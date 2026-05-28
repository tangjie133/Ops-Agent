package webhook

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type Server struct {
	cfg    *config.Config
	http   *http.Server
	logger *log.Logger
}

func NewServer(cfg *config.Config, store *todo.FileStore, onAdd OnEnqueue) *Server {
	mux := http.NewServeMux()
	handler := NewHandler(cfg, store, onAdd)
	mux.Handle(cfg.Webhook.Path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &Server{
		cfg: cfg,
		http: &http.Server{
			Addr:              cfg.Webhook.Listen,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		},
		logger: log.Default(),
	}
}

func (s *Server) Start() error {
	if !s.cfg.Webhook.Enabled {
		s.logger.Printf("webhook: disabled")
		return nil
	}
	s.logger.Printf("webhook: listening on http://%s%s (GitHub App / repo webhook)", s.cfg.Webhook.Listen, s.cfg.Webhook.Path)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("webhook: server error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) Addr() string {
	return fmt.Sprintf("http://%s%s", s.cfg.Webhook.Listen, s.cfg.Webhook.Path)
}

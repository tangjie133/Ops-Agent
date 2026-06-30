package webhook

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/todo"
)

type Server struct {
	cfg    *config.Config
	http   *http.Server
	ln     net.Listener
	logger *log.Logger
}

func NewServer(cfg *config.Config, store *todo.FileStore, onEvent OnEvent, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	mux := http.NewServeMux()
	handler := NewHandler(cfg, store, onEvent)
	path := normalizePath(cfg.Webhook.Path)
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &Server{
		cfg: cfg,
		http: &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		},
		logger: logger,
	}
}

func normalizePath(path string) string {
	if path == "" {
		return "/webhooks/github"
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

func (s *Server) Start() error {
	if !s.cfg.Webhook.Enabled {
		s.logger.Printf("webhook: disabled")
		return nil
	}

	addr := s.cfg.Webhook.Listen
	if addr == "" {
		addr = "127.0.0.1:8765"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.ln = ln
	s.http.Addr = ln.Addr().String()

	s.logger.Printf("webhook: listening on http://%s%s", ln.Addr().String(), normalizePath(s.cfg.Webhook.Path))
	go func() {
		if err := s.http.Serve(ln); err != nil && err != http.ErrServerClosed {
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
	if s.ln != nil {
		return fmt.Sprintf("http://%s%s", s.ln.Addr().String(), normalizePath(s.cfg.Webhook.Path))
	}
	return s.cfg.Webhook.LocalURL()
}

func (s *Server) HealthURL() string {
	if s.ln != nil {
		return fmt.Sprintf("http://%s/healthz", s.ln.Addr().String())
	}
	listen := s.cfg.Webhook.Listen
	if listen == "" {
		listen = "127.0.0.1:8765"
	}
	return "http://" + listen + "/healthz"
}

package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/heimdex/heimdex-agent/internal/catalog"
	"github.com/heimdex/heimdex-agent/internal/pipelines"
	"github.com/heimdex/heimdex-agent/internal/playback"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

type ServerConfig struct {
	Port           int
	ArtifactsDir   string
	CatalogService catalog.CatalogService
	PlaybackServer playback.PlaybackService
	Repository     catalog.Repository
	Runner         *catalog.Runner
	Doctor         *pipelines.CachedDoctor
	Logger         *slog.Logger
	StartTime      time.Time
	DeviceID       string
}

func NewServer(cfg ServerConfig) *Server {
	router := NewRouter(cfg)

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf("127.0.0.1:%d", cfg.Port),
			Handler:      router,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  60 * time.Second,
		},
		logger: cfg.Logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Addr() string {
	return s.httpServer.Addr
}

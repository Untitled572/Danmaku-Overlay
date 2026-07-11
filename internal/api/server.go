package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/l31155/danmaku-overlay/internal/auth"
	"github.com/l31155/danmaku-overlay/internal/config"
	"github.com/l31155/danmaku-overlay/internal/db"
	"github.com/l31155/danmaku-overlay/internal/websocket"
	"github.com/l31155/danmaku-overlay/internal/workers"
)

type Server struct {
	server  *http.Server
	dbq     *db.DBQueue
	hub     *websocket.Hub
	cfg     *config.Config
	scraper *workers.Scraper
	scanner *workers.Scanner
}

func NewServer(dbq *db.DBQueue, hub *websocket.Hub, cfg *config.Config, scraper *workers.Scraper, scanner *workers.Scanner) *Server {
	return &Server{
		dbq:     dbq,
		hub:     hub,
		cfg:     cfg,
		scraper: scraper,
		scanner: scanner,
	}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	corsHandler := s.corsMiddleware(mux)

	authMiddleware := auth.TokenAuth(s.cfg.LocalToken)

	s.registerRoutes(mux, authMiddleware)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      corsHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("HTTP server starting", "addr", addr)
	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

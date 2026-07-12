package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/l31155/danmaku-overlay/internal/auth"
	"github.com/l31155/danmaku-overlay/internal/config"
	"github.com/l31155/danmaku-overlay/internal/db"
	dlog "github.com/l31155/danmaku-overlay/internal/log"
	"github.com/l31155/danmaku-overlay/internal/websocket"
	"github.com/l31155/danmaku-overlay/internal/workers"
)

type Server struct {
	server      *http.Server
	dbq         atomic.Pointer[db.DBQueue]
	hub         *websocket.Hub
	cfg         *config.Config
	scannerMgr  atomic.Pointer[workers.ScannerManager]
	scraperPtr  atomic.Pointer[workers.Scraper]
	progress    *workers.Progress
	logCollector *dlog.Collector
	ctx         context.Context
}

func NewServer(dbq *db.DBQueue, hub *websocket.Hub, cfg *config.Config, scraper *workers.Scraper, scannerManager *workers.ScannerManager, progress *workers.Progress, logCollector *dlog.Collector, ctx context.Context) *Server {
	s := &Server{
		hub:         hub,
		cfg:         cfg,
		progress:    progress,
		logCollector: logCollector,
		ctx:         ctx,
	}
	s.dbq.Store(dbq)
	if scraper != nil {
		s.scraperPtr.Store(scraper)
	}
	if scannerManager != nil {
		s.scannerMgr.Store(scannerManager)
	}
	return s
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

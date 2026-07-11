package api

import (
	"net/http"
	"strings"

	"github.com/l31155/danmaku-overlay/internal/auth"
	"github.com/l31155/danmaku-overlay/internal/websocket"
)

func (s *Server) registerRoutes(mux *http.ServeMux, authMiddleware func(http.Handler) http.Handler) {
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	// Static route for covers
	mux.Handle("/covers/", http.StripPrefix("/covers/", http.FileServer(http.Dir(s.cfg.DataDir+"/covers"))))

	mux.Handle("/api/v1/", authMiddleware(http.HandlerFunc(s.handleProtected)))

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler := websocket.ServeWS(s.hub, auth.TokenAuth(s.cfg.LocalToken))
		wsHandler.ServeHTTP(w, r)
	})
}

func (s *Server) handleProtected(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/series":
		s.handleGetSeries(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/episodes":
		s.handleGetEpisodes(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/episodes/") && strings.HasSuffix(r.URL.Path, "/danmaku"):
		s.handleGetDanmaku(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/episodes/") && strings.HasSuffix(r.URL.Path, "/match"):
		s.handleMatchEpisode(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/progress":
		s.handleGetProgress(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/progress":
		s.handleUpdateProgress(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/scan":
		s.handleTriggerScan(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/settings":
		s.handleGetSettings(w, r)
	case r.Method == http.MethodPut && r.URL.Path == "/api/v1/settings":
		s.handleUpdateSettings(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/library":
		s.handleGetLibraries(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/library":
		s.handleCreateLibrary(w, r)
	default:
		writeError(w, "not found", http.StatusNotFound)
	}
}

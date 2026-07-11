package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/l31155/danmaku-overlay/internal/db"
)

func writeJSON(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) handleGetSeries(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")

	var series []db.Series
	err := s.dbq.Read(func(tx *gorm.DB) error {
		q := tx.Model(&db.Series{})
		if search != "" {
			q = q.Where("title LIKE ?", "%"+search+"%")
		}
		return q.Find(&series).Error
	})
	if err != nil {
		slog.Error("failed to query series", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if series == nil {
		series = []db.Series{}
	}
	writeJSON(w, series, http.StatusOK)
}

func (s *Server) handleGetEpisodes(w http.ResponseWriter, r *http.Request) {
	seriesIDStr := r.URL.Query().Get("series_id")

	var episodes []db.Episode
	err := s.dbq.Read(func(tx *gorm.DB) error {
		q := tx.Model(&db.Episode{})
		if seriesIDStr != "" {
			sid, err := strconv.ParseUint(seriesIDStr, 10, 32)
			if err != nil {
				return fmt.Errorf("parse series_id: %w", err)
			}
			q = q.Where("series_id = ?", sid)
		}
		return q.Find(&episodes).Error
	})
	if err != nil {
		slog.Error("failed to query episodes", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if episodes == nil {
		episodes = []db.Episode{}
	}
	writeJSON(w, episodes, http.StatusOK)
}

func (s *Server) handleGetDanmaku(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/episodes/")
	path = strings.TrimSuffix(path, "/danmaku")
	epIDStr := path

	epID, err := strconv.ParseUint(epIDStr, 10, 32)
	if err != nil {
		writeError(w, "invalid episode id", http.StatusBadRequest)
		return
	}

	var episode db.Episode
	err = s.dbq.Read(func(tx *gorm.DB) error {
		return tx.First(&episode, epID).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			writeError(w, "episode not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to query episode", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if episode.DanmakuPath == nil || *episode.DanmakuPath == "" {
		if s.scraper != nil {
			if err := s.scraper.DownloadDanmaku(r.Context(), &episode); err != nil {
				slog.Warn("lazy load danmaku failed", "episode", episode.RelativePath, "error", err)
			}
		}

		if episode.DanmakuPath == nil || *episode.DanmakuPath == "" {
			writeJSON(w, []any{}, http.StatusOK)
			return
		}
	}

	data, err := os.ReadFile(*episode.DanmakuPath)
	if err != nil {
		slog.Error("failed to read danmaku file", "path", *episode.DanmakuPath, "error", err)
		writeError(w, "danmaku file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleMatchEpisode(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/episodes/")
	path = strings.TrimSuffix(path, "/match")
	epIDStr := path

	epID, err := strconv.ParseUint(epIDStr, 10, 32)
	if err != nil {
		writeError(w, "invalid episode id", http.StatusBadRequest)
		return
	}

	var episode db.Episode
	err = s.dbq.Read(func(tx *gorm.DB) error {
		return tx.First(&episode, epID).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			writeError(w, "episode not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to query episode", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if s.scraper == nil {
		writeError(w, "scraper not available", http.StatusInternalServerError)
		return
	}

	if err := s.scraper.DownloadDanmaku(r.Context(), &episode); err != nil {
		slog.Error("match episode failed", "episode", episode.RelativePath, "error", err)
		writeError(w, "match failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-fetch episode to get updated fields
	s.dbq.Read(func(tx *gorm.DB) error {
		return tx.First(&episode, epID).Error
	})

	writeJSON(w, map[string]interface{}{
		"episode_id":        episode.ID,
		"dandan_episode_id": episode.DandanEpisodeID,
		"danmaku_path":      episode.DanmakuPath,
	}, http.StatusOK)
}

func (s *Server) handleGetProgress(w http.ResponseWriter, r *http.Request) {
	episodeIDStr := r.URL.Query().Get("episode_id")

	var histories []db.History
	err := s.dbq.Read(func(tx *gorm.DB) error {
		q := tx.Model(&db.History{})
		if episodeIDStr != "" {
			eid, err := strconv.ParseUint(episodeIDStr, 10, 32)
			if err != nil {
				return fmt.Errorf("parse episode_id: %w", err)
			}
			q = q.Where("episode_id = ?", eid)
		}
		return q.Find(&histories).Error
	})
	if err != nil {
		slog.Error("failed to query progress", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if histories == nil {
		histories = []db.History{}
	}
	writeJSON(w, histories, http.StatusOK)
}

type updateProgressRequest struct {
	EpisodeID uint    `json:"episode_id"`
	Position  float64 `json:"position"`
}

func (s *Server) handleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	var req updateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.EpisodeID == 0 {
		writeError(w, "episode_id is required", http.StatusBadRequest)
		return
	}

	err := s.dbq.Write(func(tx *gorm.DB) error {
		result := tx.Where(db.History{UserID: 1, EpisodeID: req.EpisodeID}).
			Assign(db.History{Position: req.Position}).
			FirstOrCreate(&db.History{})
		return result.Error
	})
	if err != nil {
		slog.Error("failed to update progress", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]bool{"ok": true}, http.StatusOK)
}

func (s *Server) handleTriggerScan(w http.ResponseWriter, r *http.Request) {
	slog.Info("scan triggered via API")
	if s.scanner != nil {
		s.scanner.TriggerScan()
	}
	if s.scraper != nil {
		go func() {
			if err := s.scraper.ScrapeAllUnmatched(r.Context()); err != nil {
				slog.Error("manual scrape failed", "error", err)
			}
		}()
	}
	writeJSON(w, map[string]string{"message": "scan and scrape triggered"}, http.StatusAccepted)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	var settings []db.Setting
	if err := s.dbq.Read(func(tx *gorm.DB) error {
		return tx.Find(&settings).Error
	}); err != nil {
		slog.Error("failed to fetch settings", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	settingsMap := make(map[string]json.RawMessage)
	for _, setting := range settings {
		settingsMap[setting.Key] = setting.Value
	}

	writeJSON(w, settingsMap, http.StatusOK)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settingsMap map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&settingsMap); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	for key, value := range settingsMap {
		var setting db.Setting
		if err := s.dbq.Write(func(tx *gorm.DB) error {
			return tx.Where(db.Setting{Key: key}).
				Assign(db.Setting{Value: value}).
				FirstOrCreate(&setting).Error
		}); err != nil {
			slog.Error("failed to update setting", "key", key, "error", err)
			writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, map[string]bool{"ok": true}, http.StatusOK)
}

func (s *Server) handleGetLibraries(w http.ResponseWriter, r *http.Request) {
	var libraries []db.Library
	if err := s.dbq.Read(func(tx *gorm.DB) error {
		return tx.Find(&libraries).Error
	}); err != nil {
		slog.Error("failed to fetch libraries", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if libraries == nil {
		libraries = []db.Library{}
	}

	writeJSON(w, libraries, http.StatusOK)
}

func (s *Server) handleCreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RootPath string `json:"root_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.RootPath == "" {
		writeError(w, "root_path is required", http.StatusBadRequest)
		return
	}

	library := db.Library{RootPath: req.RootPath}
	if err := s.dbq.Write(func(tx *gorm.DB) error {
		return tx.Create(&library).Error
	}); err != nil {
		slog.Error("failed to create library", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, library, http.StatusCreated)
}

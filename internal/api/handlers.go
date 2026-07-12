package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/l31155/danmaku-overlay/internal/db"
	"github.com/l31155/danmaku-overlay/internal/workers"
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
	dbStatus := "ready"
	if s.dbq.Load() == nil {
		dbStatus = "uninitialized"
	}
	writeJSON(w, map[string]string{
		"status":   "ok",
		"database": dbStatus,
	}, http.StatusOK)
}

func (s *Server) handleGetInitStatus(w http.ResponseWriter, r *http.Request) {
	dbq := s.dbq.Load()
	if dbq == nil {
		writeJSON(w, map[string]interface{}{
			"initialized": false,
			"status":      "uninitialized",
		}, http.StatusOK)
		return
	}
	writeJSON(w, map[string]interface{}{
		"initialized": true,
		"db_path":     s.cfg.DBPath,
		"status":      "ready",
	}, http.StatusOK)
}

func (s *Server) handleGetMigrationStatus(w http.ResponseWriter, r *http.Request) {
	marker, err := db.ReadMigrationMarker()
	if err != nil {
		writeJSON(w, map[string]string{"status": "idle"}, http.StatusOK)
		return
	}
	writeJSON(w, map[string]string{
		"status": "migrating",
		"from":   marker.From,
		"to":     marker.To,
	}, http.StatusOK)
}

func (s *Server) handleGetEpisodes(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("series_id")

	var episodes []db.Episode
	err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		q := tx.Model(&db.Episode{})
		if seriesID != "" {
			q = q.Where("series_id = ?", seriesID)
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
	episodeID := path

	var episode db.Episode
	err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Where("id = ?", episodeID).First(&episode).Error
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
		if scraper := s.scraperPtr.Load(); scraper != nil {
			if err := scraper.DownloadDanmaku(r.Context(), &episode); err != nil {
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
	episodeID := path

	var episode db.Episode
	err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Where("id = ?", episodeID).First(&episode).Error
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

	if scraper := s.scraperPtr.Load(); scraper == nil {
		writeError(w, "scraper not available", http.StatusInternalServerError)
		return
	}

	if err := s.scraperPtr.Load().DownloadDanmaku(r.Context(), &episode); err != nil {
		slog.Error("match episode failed", "episode", episode.RelativePath, "error", err)
		writeError(w, "match failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-fetch episode to get updated fields
	s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Where("id = ?", episodeID).First(&episode).Error
	})

	writeJSON(w, map[string]interface{}{
		"episode_id":        episode.ID,
		"dandan_episode_id": episode.DandanEpisodeID,
		"danmaku_path":      episode.DanmakuPath,
	}, http.StatusOK)
}

func (s *Server) handleGetProgress(w http.ResponseWriter, r *http.Request) {
	episodeID := r.URL.Query().Get("episode_id")

	var histories []db.History
	err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		q := tx.Model(&db.History{})
		if episodeID != "" {
			q = q.Where("episode_id = ?", episodeID)
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
	EpisodeID string  `json:"episode_id"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
}

func (s *Server) handleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	var req updateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.EpisodeID == "" {
		writeError(w, "episode_id is required", http.StatusBadRequest)
		return
	}

	err := s.dbq.Load().Write(func(tx *gorm.DB) error {
		result := tx.Where(db.History{UserID: 1, EpisodeID: req.EpisodeID}).
			Assign(db.History{Position: req.Position}).
			FirstOrCreate(&db.History{})
		if result.Error != nil {
			return result.Error
		}

		if req.Duration > 0 {
			watchProgress := req.Position / req.Duration
			if watchProgress > 1 {
				watchProgress = 1
			}
			return tx.Model(&db.Episode{}).Where("id = ?", req.EpisodeID).
				Update("watch_progress", watchProgress).Error
		}

		return nil
	})
	if err != nil {
		slog.Error("failed to update progress", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]bool{"ok": true}, http.StatusOK)
}

// handleTriggerScan - 只触发扫描
func (s *Server) handleTriggerScan(w http.ResponseWriter, r *http.Request) {
	slog.Info("scan triggered via API")
	sm := s.scannerMgr.Load()
	if sm == nil {
		writeError(w, "scanner not initialized, add a library first", http.StatusServiceUnavailable)
		return
	}
	sm.TriggerScan()
	writeJSON(w, map[string]string{"message": "scan triggered"}, http.StatusAccepted)
}

// handleTriggerScrape - 只触发刮削
func (s *Server) handleTriggerScrape(w http.ResponseWriter, r *http.Request) {
	slog.Info("scrape triggered via API")
	scraper := s.scraperPtr.Load()
	if scraper == nil {
		writeError(w, "scraper not initialized", http.StatusServiceUnavailable)
		return
	}
	go func() {
		if err := scraper.ScrapeAllUnmatched(s.ctx); err != nil {
			slog.Error("scrape failed", "error", err)
		}
	}()
	writeJSON(w, map[string]string{"message": "scrape triggered"}, http.StatusAccepted)
}

// handleGetStatus - 获取扫描和刮削进度
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if s.progress == nil {
		writeJSON(w, map[string]interface{}{
			"scan":   map[string]interface{}{"status": "idle"},
			"scrape": map[string]interface{}{"status": "idle"},
		}, http.StatusOK)
		return
	}
	writeJSON(w, s.progress.GetStatus(), http.StatusOK)
}

// handleGetLogs - 获取日志
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	if s.logCollector == nil {
		writeJSON(w, map[string]interface{}{
			"logs":  []interface{}{},
			"total": 0,
		}, http.StatusOK)
		return
	}

	level := r.URL.Query().Get("level")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, total := s.logCollector.ReadLogs(level, limit)
	writeJSON(w, map[string]interface{}{
		"logs":  logs,
		"total": total,
	}, http.StatusOK)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	var settings []db.Setting
	if err := s.dbq.Load().Read(func(tx *gorm.DB) error {
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
		if err := s.dbq.Load().Write(func(tx *gorm.DB) error {
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

type LibraryFileResponse struct {
	ID            string  `json:"id"`
	SeriesID      string  `json:"series_id"`
	SeriesTitle   string  `json:"series_title"`
	RelativePath  string  `json:"relative_path"`
	FileMD5       string  `json:"file_md5"`
	FileHash      string  `json:"file_hash"`
	EpIndex       *float64 `json:"ep_index"`
	MatchStatus   string  `json:"match_status"`
	ScrapeStatus  string  `json:"scrape_status"`
	WatchProgress float64 `json:"watch_progress"`
	DanmakuPath   *string `json:"danmaku_path"`
}

func (s *Server) handleGetLibraryFiles(w http.ResponseWriter, r *http.Request) {
	libraryIDStr := r.URL.Query().Get("library_id")
	if libraryIDStr == "" {
		writeError(w, "library_id is required", http.StatusBadRequest)
		return
	}
	lid, err := strconv.ParseUint(libraryIDStr, 10, 32)
	if err != nil {
		writeError(w, "invalid library_id", http.StatusBadRequest)
		return
	}

	var files []LibraryFileResponse
	err = s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Table("episodes").
			Select(`episodes.id, episodes.series_id, COALESCE(series.title, '') as series_title,
            episodes.relative_path, episodes.file_md5, episodes.file_hash,
            episodes.ep_index, episodes.match_status, episodes.scrape_status,
            episodes.watch_progress, episodes.danmaku_path`).
			Joins("LEFT JOIN series ON series.id = episodes.series_id").
			Where("episodes.library_id = ?", lid).
			Order("episodes.series_id, episodes.ep_index").
			Find(&files).Error
	})
	if err != nil {
		slog.Error("failed to query library files", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if files == nil {
		files = []LibraryFileResponse{}
	}
	writeJSON(w, files, http.StatusOK)
}

func (s *Server) handleInitLibrary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DBPath string `json:"db_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DBPath == "" {
		writeError(w, "db_path is required", http.StatusBadRequest)
		return
	}

	dir := filepath.Dir(req.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create database directory", "dir", dir, "error", err)
		writeError(w, "failed to create database directory", http.StatusInternalServerError)
		return
	}

	os.MkdirAll(s.cfg.DataDir+"/danmaku", 0755)
	os.MkdirAll(s.cfg.DataDir+"/covers", 0755)

	s.cfg.DBPath = req.DBPath
	newQueue, err := db.InitDB(s.cfg)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		writeError(w, "failed to initialize database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := db.SaveDBPathConfig(req.DBPath); err != nil {
		slog.Error("failed to save db path config", "error", err)
		writeError(w, "failed to save database path", http.StatusInternalServerError)
		newQueue.Close()
		return
	}

	s.dbq.Store(newQueue)
	slog.Info("database initialized", "path", req.DBPath)

	// Create ScannerManager and Scraper after database initialization
	var libraries []db.Library
	newQueue.Read(func(tx *gorm.DB) error {
		return tx.Find(&libraries).Error
	})

	if len(libraries) > 0 {
		sm := workers.NewScannerManager(newQueue, libraries, s.cfg.DataDir, s.progress)
		sm.Start(s.ctx)
		s.scannerMgr.Store(sm)
	}

	scraper := workers.NewScraper(newQueue, s.cfg.DataDir, s.progress)
	s.scraperPtr.Store(scraper)

	writeJSON(w, map[string]string{"db_path": req.DBPath, "message": "database initialized"}, http.StatusCreated)
}

func (s *Server) handleGetLibraries(w http.ResponseWriter, r *http.Request) {
	var libraries []db.Library
	if err := s.dbq.Load().Read(func(tx *gorm.DB) error {
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
	if err := s.dbq.Load().Write(func(tx *gorm.DB) error {
		return tx.Create(&library).Error
	}); err != nil {
		slog.Error("failed to create library", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Update ScannerManager with new library
	var libraries []db.Library
	s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Find(&libraries).Error
	})

	if len(libraries) > 0 {
		sm := workers.NewScannerManager(s.dbq.Load(), libraries, s.cfg.DataDir, s.progress)
		sm.Start(s.ctx)
		s.scannerMgr.Store(sm)
	}

	writeJSON(w, library, http.StatusCreated)
}

func (s *Server) handleDeleteLibrary(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeError(w, "id is required", http.StatusBadRequest)
		return
	}

	lid, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeError(w, "invalid library id", http.StatusBadRequest)
		return
	}

	var library db.Library
	if err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.First(&library, lid).Error
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			writeError(w, "library not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to query library", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var episodes []db.Episode
	s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Where("library_id = ?", lid).Find(&episodes).Error
	})

	danmakuPaths := make([]string, 0)
	for _, ep := range episodes {
		if ep.DanmakuPath != nil && *ep.DanmakuPath != "" {
			danmakuPaths = append(danmakuPaths, *ep.DanmakuPath)
		}
	}

	var orphanSeriesIDs []string
	s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Raw(`
			SELECT series_id FROM episodes
			WHERE series_id IN (SELECT series_id FROM episodes WHERE library_id = ?)
			GROUP BY series_id
			HAVING SUM(CASE WHEN library_id != ? THEN 1 ELSE 0 END) = 0
		`, lid, lid).Scan(&orphanSeriesIDs).Error
	})

	coverPaths := make([]string, 0)
	if len(orphanSeriesIDs) > 0 {
		var orphanSeries []db.Series
		s.dbq.Load().Read(func(tx *gorm.DB) error {
			return tx.Where("id IN ?", orphanSeriesIDs).Find(&orphanSeries).Error
		})
		for _, ser := range orphanSeries {
			if ser.CoverPath != nil && *ser.CoverPath != "" {
				coverPaths = append(coverPaths, *ser.CoverPath)
			}
		}
	}

	s.dbq.Load().Write(func(tx *gorm.DB) error {
		return tx.Where("library_id = ?", lid).Delete(&db.Episode{}).Error
	})

	if len(orphanSeriesIDs) > 0 {
		s.dbq.Load().Write(func(tx *gorm.DB) error {
			return tx.Where("id IN ?", orphanSeriesIDs).Delete(&db.Series{}).Error
		})
	}

	s.dbq.Load().Write(func(tx *gorm.DB) error {
		return tx.Delete(&db.Library{}, lid).Error
	})

	for _, p := range danmakuPaths {
		os.Remove(p)
	}
	for _, p := range coverPaths {
		os.Remove(filepath.Join(s.cfg.DataDir, p))
	}

	var libraries []db.Library
	s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Find(&libraries).Error
	})
	if len(libraries) > 0 {
		sm := workers.NewScannerManager(s.dbq.Load(), libraries, s.cfg.DataDir, s.progress)
		sm.Start(s.ctx)
		s.scannerMgr.Store(sm)
	}

	slog.Info("library deleted", "id", lid, "episodes", len(episodes), "orphan_series", len(orphanSeriesIDs))
	writeJSON(w, map[string]bool{"ok": true}, http.StatusOK)
}

// handleSearch - 增强搜索功能
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	airdate := r.URL.Query().Get("airdate")
	ratingMin := r.URL.Query().Get("rating_min")
	tags := r.URL.Query().Get("tags")

	var series []db.Series
	err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		query := tx.Model(&db.Series{})

		if q != "" {
			query = query.Where("title LIKE ? OR name_cn LIKE ?", "%"+q+"%", "%"+q+"%")
		}
		if airdate != "" {
			query = query.Where("air_date = ?", airdate)
		}
		if ratingMin != "" {
			minRating, err := strconv.ParseFloat(ratingMin, 64)
			if err == nil {
				query = query.Where("rating >= ?", minRating)
			}
		}
		if tags != "" {
			for _, tag := range strings.Split(tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					query = query.Where("tags LIKE ?", "%"+tag+"%")
				}
			}
		}

		return query.Order("air_date DESC, id ASC").Find(&series).Error
	})
	if err != nil {
		slog.Error("failed to search series", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if series == nil {
		series = []db.Series{}
	}
	writeJSON(w, map[string]interface{}{
		"series": series,
		"total":  len(series),
	}, http.StatusOK)
}

// isDanmakuEnabled 检查弹幕开关是否开启，默认不开启
func (s *Server) isDanmakuEnabled() bool {
	if s.dbq.Load() == nil {
		return false
	}
	var setting db.Setting
	if err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Where("key = ?", "danmaku_enabled").First(&setting).Error
	}); err != nil {
		return false
	}
	var enabled bool
	json.Unmarshal(setting.Value, &enabled)
	return enabled
}

// handlePlay - 开始播放
func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	episodeID := r.URL.Query().Get("episode_id")
	if episodeID == "" {
		writeError(w, "episode_id is required", http.StatusBadRequest)
		return
	}

	var episode db.Episode
	err := s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Where("id = ?", episodeID).First(&episode).Error
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

	var library db.Library
	err = s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.First(&library, episode.LibraryID).Error
	})
	if err != nil {
		slog.Error("failed to query library", "error", err)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var series db.Series
	s.dbq.Load().Read(func(tx *gorm.DB) error {
		return tx.Where("id = ?", episode.SeriesID).First(&series).Error
	})

	now := time.Now()
	s.dbq.Load().Write(func(tx *gorm.DB) error {
		return tx.Model(&episode).Update("last_played_at", now).Error
	})
	if now.After(series.LastPlayedAt) {
		s.dbq.Load().Write(func(tx *gorm.DB) error {
			return tx.Model(&series).Update("last_played_at", now).Error
		})
	}

	filePath := filepath.Join(library.RootPath, episode.RelativePath)

	danmakuLoaded := false
	if s.isDanmakuEnabled() {
		if episode.DanmakuPath != nil && *episode.DanmakuPath != "" {
			danmakuLoaded = true
		} else if scraper := s.scraperPtr.Load(); scraper != nil {
			if err := scraper.DownloadDanmaku(r.Context(), &episode); err != nil {
				slog.Warn("load danmaku failed during play", "episode", episode.RelativePath, "error", err)
			} else {
				danmakuLoaded = true
			}
		}
	}

	writeJSON(w, map[string]interface{}{
		"episode_id":     episode.ID,
		"file_path":      filePath,
		"danmaku_loaded": danmakuLoaded,
		"danmaku_path":   episode.DanmakuPath,
		"series_title":   series.Title,
		"series_name_cn": series.NameCN,
		"ep_index":       episode.EpIndex,
		"watch_progress": episode.WatchProgress,
	}, http.StatusOK)
}

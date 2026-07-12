package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/l31155/danmaku-overlay/internal/api"
	"github.com/l31155/danmaku-overlay/internal/config"
	"github.com/l31155/danmaku-overlay/internal/db"
	dlog "github.com/l31155/danmaku-overlay/internal/log"
	"github.com/l31155/danmaku-overlay/internal/websocket"
	"github.com/l31155/danmaku-overlay/internal/workers"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	// Create log collector
	logDir := filepath.Join(cfg.DataDir, "logs")
	logCollector, err := dlog.NewCollector(logDir, cfg.LogMaxDays)
	if err != nil {
		slog.Error("failed to create log collector", "error", err)
		os.Exit(1)
	}
	defer logCollector.Close()

	// Start log cleanup routine
	dlog.StartCleanupRoutine(context.Background(), logCollector, 24)

	// Create multi-handler for stderr + file
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("01-02 15:04:05"))
			}
			return a
		},
	})
	fileHandler := dlog.NewFileHandler(logCollector, cfg.LogLevel)
	multiHandler := dlog.NewMultiHandler(stderrHandler, fileHandler)

	slog.SetDefault(slog.New(multiHandler))

	slog.Info("starting Danmaku Media Core")

	// Try to load persisted db_path from .danmaku-dbpath config file
	if persistedPath, err := db.ReadDBPathConfig(); err == nil && persistedPath != "" {
		cfg.DBPath = persistedPath
		slog.Info("using persisted database path", "path", cfg.DBPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := websocket.NewHub(ctx)
	hub.Start()
	defer hub.Stop()

	// Conditional database initialization
	var queue *db.DBQueue
	var scannerManager *workers.ScannerManager
	var scraper *workers.Scraper
	progress := workers.NewProgress()

	if _, err := os.Stat(cfg.DBPath); err == nil {
		queue, err = db.InitDB(cfg)
		if err != nil {
			slog.Error("failed to initialize database", "error", err)
			os.Exit(1)
		}

		// Check if db_path setting requests migration to a new path
		var desiredPath string
		queue.Read(func(tx *gorm.DB) error {
			var s db.Setting
			if err := tx.Where("key = ?", "db_path").First(&s).Error; err != nil {
				return nil
			}
			json.Unmarshal(s.Value, &desiredPath)
			return nil
		})

		if desiredPath != "" && desiredPath != cfg.DBPath {
			slog.Info("database path changed, starting migration", "from", cfg.DBPath, "to", desiredPath)

			// 1. Write migration marker so status API reports "migrating"
			if err := db.WriteMigrationMarker(cfg.DBPath, desiredPath); err != nil {
				slog.Error("failed to write migration marker", "error", err)
				os.Exit(1)
			}

			// 2. Close current connection
			queue.Close()
			queue = nil

			// 3. Move database files
			if err := db.MigrateDBFile(cfg.DBPath, desiredPath); err != nil {
				slog.Error("database file migration failed", "error", err)
				os.Exit(1)
			}

			oldDataDir := cfg.DataDir
			newDataDir := filepath.Dir(desiredPath)
			if newDataDir != oldDataDir {
				for _, sub := range []string{"danmaku", "covers"} {
					src := oldDataDir + "/" + sub
					dst := newDataDir + "/" + sub
					if _, err := os.Stat(src); err == nil {
						if err := os.Rename(src, dst); err != nil {
							slog.Error("failed to move directory during migration", "from", src, "to", dst, "error", err)
						} else {
							slog.Info("moved directory during migration", "from", src, "to", dst)
						}
					}
				}
				cfg.DataDir = newDataDir
			}

			// 4. Persist new path
			if err := db.SaveDBPathConfig(desiredPath); err != nil {
				slog.Error("failed to persist new database path", "error", err)
				os.Exit(1)
			}
			cfg.DBPath = desiredPath

			// 5. Remove migration marker
			if err := db.RemoveMigrationMarker(); err != nil {
				slog.Warn("failed to remove migration marker", "error", err)
			}

			// 6. Re-open database at new path
			queue, err = db.InitDB(cfg)
			if err != nil {
				slog.Error("failed to re-open database after migration", "error", err)
				os.Exit(1)
			}
			slog.Info("database migration completed")
		}

		var libraries []db.Library
		queue.Read(func(tx *gorm.DB) error {
			return tx.Find(&libraries).Error
		})

		if len(libraries) > 0 {
			scannerManager = workers.NewScannerManager(queue, libraries, cfg.DataDir, progress)
			if err := scannerManager.Start(ctx); err != nil {
				slog.Error("failed to start scanner manager", "error", err)
			}
		}

		scraper = workers.NewScraper(queue, cfg.DataDir, progress)

		if scannerManager != nil {
			scrapeTriggerCh := make(chan struct{}, 1)
			scannerManager.OnNewEpisode = func(ep *db.Episode) {
				select {
				case scrapeTriggerCh <- struct{}{}:
				default:
				}
			}

			go func() {
				time.Sleep(5 * time.Second)
				slog.Info("starting initial background scraping")
				if err := scraper.ScrapeAllUnmatched(ctx); err != nil {
					slog.Error("initial scraping failed", "error", err)
				}

			scanIntervalHours := 24
			queue.Read(func(tx *gorm.DB) error {
				var setting db.Setting
				if err := tx.Where("key = ?", "scan_interval_hours").First(&setting).Error; err == nil {
					json.Unmarshal(setting.Value, &scanIntervalHours)
				}
				return nil
			})
			if scanIntervalHours <= 0 {
				scanIntervalHours = 24
			}

			ticker := time.NewTicker(time.Duration(scanIntervalHours) * time.Hour)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					slog.Info("scheduled background scraping started", "interval_hours", scanIntervalHours)
					if err := scraper.ScrapeAllUnmatched(ctx); err != nil {
						slog.Error("scheduled scraping failed", "error", err)
					}
				case <-scrapeTriggerCh:
					time.Sleep(2 * time.Second)
					slog.Info("real-time background scraping started (triggered by file change)")
					if err := scraper.ScrapeAllUnmatched(ctx); err != nil {
						slog.Error("real-time scraping failed", "error", err)
					}
				}
			}
			}()
		}
	} else {
		slog.Info("no database found, waiting for POST /api/v1/library/init to create one")
	}

	apiServer := api.NewServer(queue, hub, cfg, scraper, scannerManager, progress, logCollector, ctx)

	go func() {
		addr := ":" + cfg.Port
		if err := apiServer.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP:
			slog.Info("reloading configuration")
			cfg = config.Load()
			slog.Info("configuration reloaded")
		case syscall.SIGINT, syscall.SIGTERM:
			slog.Info("shutting down...")

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if err := apiServer.Stop(shutdownCtx); err != nil {
				slog.Error("HTTP server shutdown error", "error", err)
			}

			if scannerManager != nil {
				scannerManager.Stop()
			}
			hub.Stop()
			if queue != nil {
				queue.Close()
			}

			slog.Info("shutdown complete")
			return
		}
	}
}

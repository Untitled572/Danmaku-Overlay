package db

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// MigrateV2 executes v2 database migration
func MigrateV2(db *gorm.DB) error {
	slog.Debug("running v2 migration")

	// Helper function to check if column exists
	columnExists := func(table, column string) bool {
		var count int
		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", table, column)).Scan(&count)
		return count > 0
	}

	// 1. Add Episode new fields (only if not exists)
	if !columnExists("episodes", "scrape_status") {
		if err := db.Exec(`ALTER TABLE episodes ADD COLUMN scrape_status TEXT DEFAULT 'unscraped'`).Error; err != nil {
			slog.Warn("failed to add scrape_status column", "error", err)
		}
	}
	if !columnExists("episodes", "watch_progress") {
		if err := db.Exec(`ALTER TABLE episodes ADD COLUMN watch_progress REAL DEFAULT 0`).Error; err != nil {
			slog.Warn("failed to add watch_progress column", "error", err)
		}
	}

	// 2. Add Series new fields (only if not exists)
	if !columnExists("series", "current_ep") {
		if err := db.Exec(`ALTER TABLE series ADD COLUMN current_ep INTEGER`).Error; err != nil {
			slog.Warn("failed to add current_ep column", "error", err)
		}
	}
	if !columnExists("series", "rating") {
		if err := db.Exec(`ALTER TABLE series ADD COLUMN rating REAL`).Error; err != nil {
			slog.Warn("failed to add rating column", "error", err)
		}
	}
	if !columnExists("series", "tags") {
		if err := db.Exec(`ALTER TABLE series ADD COLUMN tags TEXT`).Error; err != nil {
			slog.Warn("failed to add tags column", "error", err)
		}
	}

	// 3. Add LastPlayedAt fields (only if not exists)
	if !columnExists("episodes", "last_played_at") {
		if err := db.Exec(`ALTER TABLE episodes ADD COLUMN last_played_at DATETIME`).Error; err != nil {
			slog.Warn("failed to add last_played_at column to episodes", "error", err)
		}
	}
	if !columnExists("series", "last_played_at") {
		if err := db.Exec(`ALTER TABLE series ADD COLUMN last_played_at DATETIME`).Error; err != nil {
			slog.Warn("failed to add last_played_at column to series", "error", err)
		}
	}

	// 4. Migrate existing data: map match_status to scrape_status
	// Only update rows where scrape_status is still default/empty
	db.Exec(`UPDATE episodes SET scrape_status = 'completed' WHERE match_status = 'matched' AND (scrape_status IS NULL OR scrape_status = '' OR scrape_status = 'unscraped')`)
	db.Exec(`UPDATE episodes SET scrape_status = 'unscraped' WHERE match_status = 'unmatched' AND (scrape_status IS NULL OR scrape_status = '')`)

	// 4. Create indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_episodes_scrape_status ON episodes(scrape_status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_episodes_watch_progress ON episodes(watch_progress)`)

	slog.Info("v2 migration completed")
	return nil
}

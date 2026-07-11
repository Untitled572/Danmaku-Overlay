package db

import (
	"fmt"
	"log/slog"

	sqlite "github.com/ncruces/go-sqlite3/gormlite"
	gorm "gorm.io/gorm"

	"github.com/l31155/danmaku-overlay/internal/config"
)

func InitDB(cfg *config.Config) (*DBQueue, error) {
	dsn := "file:" + cfg.DBPath + "?cache=shared&mode=rwc"

	slog.Info("initializing database", "path", cfg.DBPath)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}

	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("set foreign_keys: %w", err)
	}

	if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	if err := db.AutoMigrate(&Library{}, &Series{}, &Episode{}, &History{}, &Setting{}); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	slog.Info("database migration completed")

	return NewDBQueue(db), nil
}

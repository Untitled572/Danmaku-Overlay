package db

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	sqlite "github.com/ncruces/go-sqlite3/gormlite"
	gorm "gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"github.com/l31155/danmaku-overlay/internal/config"
)

const dbPathConfigFile = ".danmaku-dbpath"

type nopWriter struct{}

func (nopWriter) Printf(string, ...interface{}) {}

func InitDB(cfg *config.Config) (*DBQueue, error) {
	dsn := "file:" + cfg.DBPath + "?cache=shared&mode=rwc"

	slog.Info("initializing database", "path", cfg.DBPath)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: glogger.New(
			&nopWriter{},
			glogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  glogger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(5)
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

	if err := MigrateV2(db); err != nil {
		slog.Warn("v2 migration failed (may already be applied)", "error", err)
	}

	slog.Info("database migration completed")

	return NewDBQueue(db), nil
}

type dbPathConfig struct {
	DBPath string `json:"db_path"`
}

func ReadDBPathConfig() (string, error) {
	data, err := os.ReadFile(dbPathConfigFile)
	if err != nil {
		return "", err
	}
	var cfg dbPathConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse db path config: %w", err)
	}
	return cfg.DBPath, nil
}

func SaveDBPathConfig(path string) error {
	cfg := dbPathConfig{DBPath: path}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal db path config: %w", err)
	}
	if err := os.WriteFile(dbPathConfigFile, data, 0644); err != nil {
		return fmt.Errorf("write db path config: %w", err)
	}
	return nil
}

const migrationMarkerFile = ".danmaku-migrating"

type MigrationMarker struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func WriteMigrationMarker(from, to string) error {
	m := MigrationMarker{From: from, To: to}
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal migration marker: %w", err)
	}
	if err := os.WriteFile(migrationMarkerFile, data, 0644); err != nil {
		return fmt.Errorf("write migration marker: %w", err)
	}
	return nil
}

func ReadMigrationMarker() (*MigrationMarker, error) {
	data, err := os.ReadFile(migrationMarkerFile)
	if err != nil {
		return nil, err
	}
	var m MigrationMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse migration marker: %w", err)
	}
	return &m, nil
}

func RemoveMigrationMarker() error {
	if err := os.Remove(migrationMarkerFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func MigrateDBFile(from, to string) error {
	for _, ext := range []string{"", "-wal", "-shm"} {
		src := from + ext
		dst := to + ext
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("rename %s -> %s: %w", src, dst, err)
			}
			slog.Info("moved database file", "from", src, "to", dst)
		}
	}
	return nil
}

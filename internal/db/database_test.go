package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l31155/danmaku-overlay/internal/config"
)

func TestInitDBCreatesFileAndDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	cfg := &config.Config{DBPath: dbPath}

	q, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer q.Close()

	fi, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		entries, _ := os.ReadDir(dir)
		t.Fatalf("database file not found at %s; dir contents: %v", dbPath, entries)
	}
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if fi.Size() == 0 {
		t.Error("database file exists but is empty")
	}
}

func TestInitDBJournalModeWAL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wal_check.db")

	cfg := &config.Config{DBPath: dbPath}
	q, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer q.Close()

	var journalMode string
	row := q.db.Raw("PRAGMA journal_mode").Row()
	if err := row.Scan(&journalMode); err != nil {
		t.Fatalf("failed to query PRAGMA journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}
}

func TestInitDBForeignKeysOn(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fk_check.db")

	cfg := &config.Config{DBPath: dbPath}
	q, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer q.Close()

	var fkEnabled string
	row := q.db.Raw("PRAGMA foreign_keys").Row()
	if err := row.Scan(&fkEnabled); err != nil {
		t.Fatalf("failed to query PRAGMA foreign_keys: %v", err)
	}
	if fkEnabled != "1" {
		t.Errorf("foreign_keys = %q, want %q", fkEnabled, "1")
	}
}

func TestInitDBBusyTimeout(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "busy_check.db")

	cfg := &config.Config{DBPath: dbPath}
	q, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer q.Close()

	var busyTimeout string
	row := q.db.Raw("PRAGMA busy_timeout").Row()
	if err := row.Scan(&busyTimeout); err != nil {
		t.Fatalf("failed to query PRAGMA busy_timeout: %v", err)
	}
	if busyTimeout != "5000" {
		t.Errorf("busy_timeout = %q, want %q", busyTimeout, "5000")
	}
}

func TestInitDBAllTablesCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tables_check.db")

	cfg := &config.Config{DBPath: dbPath}
	q, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer q.Close()

	expected := map[string]bool{
		"libraries": true,
		"series":    true,
		"episodes":  true,
		"history":   true,
		"settings":  true,
	}

	sqlDB, err := q.db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}

	rows, err := sqlDB.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatalf("failed to query tables: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		delete(expected, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration error: %v", err)
	}

	for table := range expected {
		t.Errorf("table %q was not created by InitDB", table)
	}
}

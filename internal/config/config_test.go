package config

import (
	"log/slog"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	if cfg.DBPath != "data/danmaku.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "data/danmaku.db")
	}
	if cfg.DataDir != "data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "data")
	}
	if cfg.LocalToken != "" {
		t.Errorf("LocalToken = %q, want empty", cfg.LocalToken)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.LogFilePath != "" {
		t.Errorf("LogFilePath = %q, want empty", cfg.LogFilePath)
	}
}

func TestLoadWithEnv(t *testing.T) {
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("DATA_DIR", "/tmp/data")
	t.Setenv("APP_LOCAL_TOKEN", "secret-token")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FILE", "/tmp/test.log")

	cfg := Load()

	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/test.db")
	}
	if cfg.DataDir != "/tmp/data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "/tmp/data")
	}
	if cfg.LocalToken != "secret-token" {
		t.Errorf("LocalToken = %q, want %q", cfg.LocalToken, "secret-token")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
	if cfg.LogFilePath != "/tmp/test.log" {
		t.Errorf("LogFilePath = %q, want %q", cfg.LogFilePath, "/tmp/test.log")
	}
}

func TestLoadWithEmptyEnvFallsBack(t *testing.T) {
	t.Setenv("DB_PATH", "")
	t.Setenv("APP_LOCAL_TOKEN", "")
	t.Setenv("LOG_FILE", "")

	cfg := Load()

	if cfg.DBPath != "data/danmaku.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "data/danmaku.db")
	}
	if cfg.LocalToken != "" {
		t.Errorf("LocalToken = %q, want empty", cfg.LocalToken)
	}
	if cfg.LogFilePath != "" {
		t.Errorf("LogFilePath = %q, want empty", cfg.LogFilePath)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"Debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
		{"123", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

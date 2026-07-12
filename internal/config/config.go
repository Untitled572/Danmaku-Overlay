package config

import (
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	DBPath      string
	DataDir     string
	LocalToken  string
	LogLevel    slog.Level
	LogFilePath string
	Port        string
}

func Load() *Config {
	cfg := &Config{
		DBPath:   getEnv("DB_PATH", "data/danmaku.db"),
		DataDir:  getEnv("DATA_DIR", "data"),
		LogLevel: parseLogLevel(getEnv("LOG_LEVEL", "info")),
		Port:     getEnv("PORT", "8085"),
	}

	if token := os.Getenv("APP_LOCAL_TOKEN"); token != "" {
		cfg.LocalToken = token
	}

	if logFile := os.Getenv("LOG_FILE"); logFile != "" {
		cfg.LogFilePath = logFile
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

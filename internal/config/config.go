package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DBPath            string
	DataDir           string
	LocalToken        string
	LogLevel          slog.Level
	LogFilePath       string
	LogMaxDays        int
	Port              string
	CORSAllowedOrigins []string `json:"cors_allowed_origins"`
}

func Load() *Config {
	cfg := &Config{
		DBPath:    getEnv("DB_PATH", "data/danmaku.db"),
		DataDir:   getEnv("DATA_DIR", "data"),
		LogLevel:  parseLogLevel(getEnv("LOG_LEVEL", "info")),
		LogMaxDays: getEnvInt("LOG_MAX_DAYS", 7),
		Port:      getEnv("PORT", "8085"),
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

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
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

package log

import (
	"context"
	"log/slog"
	"time"
)

// StartCleanupRoutine starts a goroutine that cleans up old logs periodically
func StartCleanupRoutine(ctx context.Context, collector *Collector, intervalHours int) {
	go func() {
		// Run cleanup on startup
		collector.Cleanup()
		slog.Info("log cleanup completed")

		ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collector.Cleanup()
				slog.Info("log cleanup completed")
			}
		}
	}()
}

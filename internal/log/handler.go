package log

import (
	"context"
	"log/slog"
	"time"
)

type FileHandler struct {
	collector *Collector
	level     slog.Level
}

func NewFileHandler(collector *Collector, level slog.Level) *FileHandler {
	return &FileHandler{
		collector: collector,
		level:     level,
	}
}

func (h *FileHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *FileHandler) Handle(ctx context.Context, r slog.Record) error {
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]string),
	}

	r.Attrs(func(a slog.Attr) bool {
		entry.Attrs[a.Key] = a.Value.String()
		return true
	})

	return h.collector.Write(entry)
}

func (h *FileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *FileHandler) WithGroup(name string) slog.Handler {
	return h
}

// MultiHandler writes to both stderr and file
type MultiHandler struct {
	stderr slog.Handler
	file   *FileHandler
	level  slog.Level
}

func NewMultiHandler(stderr slog.Handler, file *FileHandler) *MultiHandler {
	return &MultiHandler{
		stderr: stderr,
		file:   file,
		level:  file.level,
	}
}

func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	// Write to stderr
	h.stderr.Handle(ctx, r)

	// Write to file
	return h.file.Handle(ctx, r)
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MultiHandler{
		stderr: h.stderr.WithAttrs(attrs),
		file:   h.file,
		level:  h.level,
	}
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	return &MultiHandler{
		stderr: h.stderr.WithGroup(name),
		file:   h.file,
		level:  h.level,
	}
}

// Helper function to format duration
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

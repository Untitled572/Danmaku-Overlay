package log

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type LogEntry struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Message string            `json:"msg"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

type Collector struct {
	logDir  string
	maxDays int
	mu      sync.Mutex
	file    *os.File
	writer  *bufio.Writer
}

func NewCollector(logDir string, maxDays int) (*Collector, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	c := &Collector{
		logDir:  logDir,
		maxDays: maxDays,
	}

	if err := c.openFile(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Collector) openFile() error {
	filename := filepath.Join(c.logDir, fmt.Sprintf("danmaku-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	c.file = f
	c.writer = bufio.NewWriter(f)
	return nil
}

func (c *Collector) Write(entry LogEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need a new file (day changed)
	today := time.Now().Format("2006-01-02")
	currentFile := filepath.Base(c.file.Name())
	if !strings.Contains(currentFile, today) {
		c.writer.Flush()
		c.file.Close()
		if err := c.openFile(); err != nil {
			return err
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = c.writer.Write(data)
	if err != nil {
		return err
	}

	return c.writer.Flush()
}

func (c *Collector) ReadLogs(level string, limit int) ([]LogEntry, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var entries []LogEntry

	// Read all log files sorted by name (newest first)
	files, _ := filepath.Glob(filepath.Join(c.logDir, "danmaku-*.log"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	for _, filename := range files {
		entries = c.readFile(filename, level, entries, limit)
		if limit > 0 && len(entries) >= limit {
			break
		}
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, len(entries)
}

func (c *Collector) readFile(filename, level string, entries []LogEntry, limit int) []LogEntry {
	f, err := os.Open(filename)
	if err != nil {
		return entries
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if limit > 0 && len(entries) >= limit {
			break
		}

		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if level != "" && !strings.EqualFold(entry.Level, level) {
			continue
		}

		entries = append(entries, entry)
	}

	return entries
}

func (c *Collector) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

_cutoff := time.Now().AddDate(0, 0, -c.maxDays)
	files, _ := filepath.Glob(filepath.Join(c.logDir, "danmaku-*.log"))

	for _, filename := range files {
		base := filepath.Base(filename)
		dateStr := strings.TrimPrefix(base, "danmaku-")
		dateStr = strings.TrimSuffix(dateStr, ".log")

		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if date.Before(_cutoff) {
			os.Remove(filename)
		}
	}
}

func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writer != nil {
		c.writer.Flush()
	}
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

// ReadTodayLogs reads logs from today's file only (for quick access)
func (c *Collector) ReadTodayLogs(level string, limit int) ([]LogEntry, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	filename := filepath.Join(c.logDir, fmt.Sprintf("danmaku-%s.log", time.Now().Format("2006-01-02")))
	entries := c.readFile(filename, level, nil, limit)
	return entries, len(entries)
}

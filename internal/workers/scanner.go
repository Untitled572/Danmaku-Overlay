package workers

import (
	"bufio"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/l31155/danmaku-overlay/internal/db"
)

var supportedExts = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
}

type Scanner struct {
	db        *db.DBQueue
	libraryID uint
	rootPath  string
	dataDir   string
	watcher   *fsnotify.Watcher
	hashCache *lruCache
	scanCh       chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	OnNewEpisode func(ep *db.Episode)
	progress     *Progress
}

func NewScanner(dbq *db.DBQueue, libID uint, rootPath, dataDir string, progress *Progress) *Scanner {
	return &Scanner{
		db:        dbq,
		libraryID: libID,
		rootPath:  rootPath,
		dataDir:   dataDir,
		hashCache: newLRUCache(1000),
		scanCh:    make(chan struct{}, 1),
		progress:  progress,
	}
}

func (s *Scanner) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	s.watcher = w

	if err := s.addDirRecursive(s.rootPath); err != nil {
		s.watcher.Close()
		return fmt.Errorf("add watch directories: %w", err)
	}

	go s.run()

	s.TriggerScan()
	return nil
}

func (s *Scanner) Stop() error {
	if s.watcher != nil {
		if err := s.watcher.Close(); err != nil {
			return fmt.Errorf("close watcher: %w", err)
		}
	}
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *Scanner) TriggerScan() {
	select {
	case s.scanCh <- struct{}{}:
	default:
	}
}

// ScannerManager manages Scanner instances for multiple Libraries
type ScannerManager struct {
	scanners     []*Scanner
	db           *db.DBQueue
	dataDir      string
	ctx          context.Context
	cancel       context.CancelFunc
	OnNewEpisode func(ep *db.Episode)
	Progress     *Progress
}

// NewScannerManager creates a ScannerManager
func NewScannerManager(dbq *db.DBQueue, libraries []db.Library, dataDir string, progress *Progress) *ScannerManager {
	sm := &ScannerManager{
		db:       dbq,
		dataDir:  dataDir,
		Progress: progress,
	}
	for _, lib := range libraries {
		scanner := NewScanner(dbq, lib.ID, lib.RootPath, dataDir, progress)
		sm.scanners = append(sm.scanners, scanner)
	}
	return sm
}

// Start starts all Scanners
func (sm *ScannerManager) Start(ctx context.Context) error {
	sm.ctx, sm.cancel = context.WithCancel(ctx)
	for _, scanner := range sm.scanners {
		scanner.OnNewEpisode = sm.OnNewEpisode
		if err := scanner.Start(sm.ctx); err != nil {
			slog.Error("failed to start scanner", "library_id", scanner.libraryID, "error", err)
		}
	}
	return nil
}

// Stop stops all Scanners
func (sm *ScannerManager) Stop() error {
	if sm.cancel != nil {
		sm.cancel()
	}
	for _, scanner := range sm.scanners {
		scanner.Stop()
	}
	return nil
}

// TriggerScan triggers all Scanners to scan
func (sm *ScannerManager) TriggerScan() {
	for _, scanner := range sm.scanners {
		scanner.TriggerScan()
	}
}

func (s *Scanner) run() {
	var (
		debounceTimer *time.Timer
		debounceCh    <-chan time.Time
		pendingEvents []fsnotify.Event
	)

	for {
		select {
		case <-s.ctx.Done():
			return

		case <-s.scanCh:
			if err := s.scanFull(s.ctx); err != nil {
				slog.Error("full scan failed", "error", err)
			}

		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if !s.shouldHandleEvent(event) {
				continue
			}

			pendingEvents = append(pendingEvents, event)
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(300 * time.Millisecond)
			debounceCh = debounceTimer.C

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)

		case <-debounceCh:
			for _, e := range pendingEvents {
				s.handleEvent(s.ctx, e)
			}
			pendingEvents = nil
			debounceCh = nil
		}
	}
}

func (s *Scanner) shouldHandleEvent(event fsnotify.Event) bool {
	if event.Op&fsnotify.Chmod != 0 {
		return false
	}
	name := filepath.Base(event.Name)
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~") {
		return false
	}
	return event.Op&(fsnotify.Create|fsnotify.Write) != 0
}

func (s *Scanner) handleEvent(ctx context.Context, event fsnotify.Event) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	info, err := os.Stat(event.Name)
	if err != nil {
		return
	}

	if info.IsDir() {
		if event.Op&fsnotify.Create != 0 {
			if err := s.addDirRecursive(event.Name); err != nil {
				slog.Warn("failed to watch new directory", "path", event.Name, "error", err)
			}
		}
		s.scanPath(ctx, event.Name)
		return
	}

	ext := strings.ToLower(filepath.Ext(event.Name))
	if !supportedExts[ext] {
		return
	}

	s.processFile(ctx, event.Name)
}

func (s *Scanner) scanFull(ctx context.Context) error {
	start := time.Now()
	var totalFiles, newFiles, updatedFiles, skippedFiles int

	slog.Info("scan started", "root", s.rootPath)

	existingFiles := make(map[string]string) // relativePath -> fileHash
	s.db.Read(func(tx *gorm.DB) error {
		var episodes []db.Episode
		tx.Where("library_id = ?", s.libraryID).Find(&episodes)
		for _, ep := range episodes {
			existingFiles[ep.RelativePath] = ep.FileHash
		}
		return nil
	})

	// First pass: count total files
	var fileCount int
	filepath.WalkDir(s.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if supportedExts[ext] {
			fileCount++
		}
		return nil
	})

	if s.progress != nil {
		s.progress.SetScanRunning(fileCount)
	}

	err := filepath.WalkDir(s.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("walk error", "path", path, "error", err)
			return nil
		}

		if d.IsDir() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExts[ext] {
			return nil
		}

		totalFiles++

		if s.progress != nil {
			s.progress.UpdateScanProgress(totalFiles, fileCount)
		}

		relPath, _ := filepath.Rel(s.rootPath, path)

		if _, exists := existingFiles[relPath]; exists {
			skippedFiles++
			return nil
		}

		if err := s.processFile(ctx, path); err != nil {
			slog.Warn("failed to process file", "path", path, "error", err)
			return nil
		}
		newFiles++

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk root path: %w", err)
	}

	elapsed := time.Since(start)
	slog.Info("scan completed",
		"total", totalFiles,
		"new", newFiles,
		"updated", updatedFiles,
		"skipped", skippedFiles,
		"elapsed", elapsed.String(),
	)

	if s.progress != nil {
		s.progress.SetScanCompleted(fmt.Sprintf("%d files, %d new, %s", totalFiles, newFiles, elapsed.Round(time.Millisecond).String()))
	}

	return nil
}

func (s *Scanner) scanPath(ctx context.Context, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExts[ext] {
			return nil
		}

		return s.processFile(ctx, path)
	})
}

func (s *Scanner) processFile(ctx context.Context, absPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if isFileLocked(absPath) {
		slog.Debug("file is locked, skipping", "path", absPath)
		return nil
	}

	relPath, err := filepath.Rel(s.rootPath, absPath)
	if err != nil {
		return fmt.Errorf("compute relative path: %w", err)
	}

	hash, err := s.getOrComputeHash(ctx, absPath)
	if err != nil {
		return fmt.Errorf("compute xxhash: %w", err)
	}

	md5Str, err := computeMD5ForDanDan(absPath)
	if err != nil {
		return fmt.Errorf("compute md5: %w", err)
	}

	episode := db.Episode{
		ID:              fmt.Sprintf("%016x", hash),
		LibraryID:       s.libraryID,
		RelativePath:    relPath,
		FileMD5:         md5Str,
		FileHash:        fmt.Sprintf("%016x", hash),
		DandanEpisodeID: 0,
		MatchStatus:     "unmatched",
		ScrapeStatus:    "unscraped",
		WatchProgress:   0,
	}

	if err := s.db.Write(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&episode)
		if result.Error != nil {
			return fmt.Errorf("insert episode: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			slog.Warn("duplicate file skipped", "path", relPath)
		} else {
			slog.Info("scanned file", "path", relPath, "hash", episode.FileHash)
			if s.OnNewEpisode != nil {
				go s.OnNewEpisode(&episode)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *Scanner) getOrComputeHash(ctx context.Context, path string) (uint64, error) {
	if h, ok := s.hashCache.Get(path); ok {
		return h, nil
	}

	h, err := computeXXHash(ctx, path)
	if err != nil {
		return 0, err
	}

	s.hashCache.Set(path, h)
	return h, nil
}

func (s *Scanner) addDirRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if watchErr := s.watcher.Add(path); watchErr != nil {
				slog.Warn("failed to watch directory", "path", path, "error", watchErr)
			}
		}
		return nil
	})
}

func computeXXHash(ctx context.Context, path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := xxhash.New()
	r := bufio.NewReaderSize(f, 64*1024)

	if _, err := io.Copy(h, r); err != nil {
		return 0, fmt.Errorf("read file: %w", err)
	}

	return h.Sum64(), nil
}

func computeMD5ForDanDan(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}

	chunkSize := int64(16 * 1024 * 1024)

	firstSize := chunkSize
	if stat.Size() < firstSize {
		firstSize = stat.Size()
	}
	first := make([]byte, firstSize)
	if _, err := io.ReadFull(f, first); err != nil {
		return "", fmt.Errorf("read first chunk: %w", err)
	}

	lastSize := chunkSize
	if stat.Size() < lastSize {
		lastSize = stat.Size()
	}
	last := make([]byte, lastSize)
	if _, err := f.ReadAt(last, stat.Size()-lastSize); err != nil {
		return "", fmt.Errorf("read last chunk: %w", err)
	}

	combined := append(first, last...)
	hash := md5.Sum(combined)
	return fmt.Sprintf("%x", hash), nil
}

func isFileLocked(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return true
	}

	unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return false
}

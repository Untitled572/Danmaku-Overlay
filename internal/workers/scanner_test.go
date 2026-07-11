package workers

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	sqlite "github.com/ncruces/go-sqlite3/gormlite"
	"golang.org/x/sys/unix"
	"gorm.io/gorm"

	"github.com/l31155/danmaku-overlay/internal/db"
)

func setupTestDB(t *testing.T) (*gorm.DB, *db.DBQueue, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	dsn := "file:" + dbPath + "?cache=shared&mode=rwc"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := gdb.AutoMigrate(&db.Library{}, &db.Series{}, &db.Episode{}, &db.History{}, &db.Setting{}); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	return gdb, db.NewDBQueue(gdb), dir
}

func writeFileOfSize(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = byte(i)
	}

	var written int64
	for written < size {
		n := int64(len(chunk))
		if rem := size - written; rem < n {
			n = rem
		}
		if _, err := f.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
		written += n
	}
}

func TestComputeXXHash(t *testing.T) {
	content := []byte("hello world, this is a deterministic test content for xxhash")
	tmpFile := filepath.Join(t.TempDir(), "hash.bin")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := computeXXHash(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("computeXXHash failed: %v", err)
	}

	expected := xxhash.Sum64(content)
	if hash != expected {
		t.Errorf("hash = %d, want %d", hash, expected)
	}
}

func TestComputeXXHash_EmptyFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "empty.bin")
	if err := os.WriteFile(tmpFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := computeXXHash(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("computeXXHash failed: %v", err)
	}

	expected := xxhash.Sum64([]byte{})
	if hash != expected {
		t.Errorf("hash = %d, want %d", hash, expected)
	}
}

func TestComputeMD5ForDanDan_SmallFile(t *testing.T) {
	content := []byte("hello world, small file content for md5")
	tmpFile := filepath.Join(t.TempDir(), "small.bin")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	md5Str, err := computeMD5ForDanDan(tmpFile)
	if err != nil {
		t.Fatalf("computeMD5ForDanDan failed: %v", err)
	}

	combined := append(content, content...)
	expected := fmt.Sprintf("%x", md5.Sum(combined))
	if md5Str != expected {
		t.Errorf("md5 = %s, want %s", md5Str, expected)
	}
}

func TestComputeMD5ForDanDan_LargeFile(t *testing.T) {
	size := int64(33 * 1024 * 1024)
	tmpFile := filepath.Join(t.TempDir(), "large.bin")
	writeFileOfSize(t, tmpFile, size)

	md5Str, err := computeMD5ForDanDan(tmpFile)
	if err != nil {
		t.Fatalf("computeMD5ForDanDan failed: %v", err)
	}

	if len(md5Str) != 32 {
		t.Errorf("md5 length = %d, want 32", len(md5Str))
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	chunkSize := int64(16 * 1024 * 1024)
	first := make([]byte, chunkSize)
	if _, err := io.ReadFull(f, first); err != nil {
		t.Fatal(err)
	}
	last := make([]byte, chunkSize)
	if _, err := f.ReadAt(last, size-chunkSize); err != nil {
		t.Fatal(err)
	}

	combined := append(first, last...)
	expected := fmt.Sprintf("%x", md5.Sum(combined))
	if md5Str != expected {
		t.Errorf("md5 = %s, want %s", md5Str, expected)
	}
}

func TestIsFileLocked_NonExistent(t *testing.T) {
	if isFileLocked("/nonexistent/path/file.txt") {
		t.Error("isFileLocked returned true for non-existent file")
	}
}

func TestIsFileLocked_Unlocked(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "unlocked.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if isFileLocked(tmpFile) {
		t.Error("isFileLocked returned true for unlocked file")
	}
}

func TestIsFileLocked_Locked(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "locked.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		t.Skipf("failed to acquire exclusive lock: %v", err)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)

	if !isFileLocked(tmpFile) {
		t.Error("isFileLocked returned false for locked file")
	}
}

func TestProcessFile(t *testing.T) {
	gdb, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	s := NewScanner(dbq, 1, dir, dir)

	videoPath := filepath.Join(dir, "test_video.mp4")
	content := []byte("fake mp4 video content for scanning test")
	if err := os.WriteFile(videoPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := s.processFile(context.Background(), videoPath); err != nil {
		t.Fatalf("processFile failed: %v", err)
	}

	var episodes []db.Episode
	if err := gdb.Find(&episodes).Error; err != nil {
		t.Fatalf("query episodes failed: %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}

	ep := episodes[0]
	if ep.LibraryID != 1 {
		t.Errorf("LibraryID = %d, want 1", ep.LibraryID)
	}
	if ep.RelativePath != "test_video.mp4" {
		t.Errorf("RelativePath = %q, want %q", ep.RelativePath, "test_video.mp4")
	}
	if ep.FileMD5 == "" {
		t.Error("FileMD5 is empty")
	}
	if ep.FileHash == "" {
		t.Error("FileHash is empty")
	}
	if ep.MatchStatus != "unmatched" {
		t.Errorf("MatchStatus = %q, want %q", ep.MatchStatus, "unmatched")
	}
}

func TestNewScannerAndStart(t *testing.T) {
	_, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	s := NewScanner(dbq, 1, dir, dir)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestScanFull(t *testing.T) {
	gdb, dbq, dir := setupTestDB(t)
	defer dbq.Close()

	video1 := filepath.Join(dir, "video1.mp4")
	os.WriteFile(video1, []byte("unique content one"), 0644)

	video2 := filepath.Join(dir, "video2.mkv")
	os.WriteFile(video2, []byte("unique content two"), 0644)

	video3 := filepath.Join(dir, "video3.mp4")
	os.WriteFile(video3, []byte("unique content one"), 0644)

	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text file"), 0644)
	os.WriteFile(filepath.Join(dir, "image.jpg"), []byte("image file"), 0644)

	subDir := filepath.Join(dir, "sub")
	os.Mkdir(subDir, 0755)
	subVideo := filepath.Join(subDir, "subvideo.webm")
	os.WriteFile(subVideo, []byte("sub content unique"), 0644)

	s := NewScanner(dbq, 1, dir, dir)

	ctx := context.Background()
	if err := s.scanFull(ctx); err != nil {
		t.Fatalf("scanFull failed: %v", err)
	}

	var episodes []db.Episode
	if err := gdb.Find(&episodes).Error; err != nil {
		t.Fatalf("query episodes failed: %v", err)
	}

	if len(episodes) != 3 {
		t.Fatalf("expected 3 episodes (2 unique video files + subdirectory), got %d", len(episodes))
	}

	paths := make(map[string]bool)
	for _, ep := range episodes {
		paths[ep.RelativePath] = true
		if ep.MatchStatus != "unmatched" {
			t.Errorf("MatchStatus for %q = %q, want %q", ep.RelativePath, ep.MatchStatus, "unmatched")
		}
	}

	if paths["readme.txt"] {
		t.Error("non-video file readme.txt was added to episodes")
	}
	if paths["image.jpg"] {
		t.Error("non-video file image.jpg was added to episodes")
	}
}

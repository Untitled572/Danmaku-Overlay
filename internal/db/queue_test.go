package db

import (
	"fmt"
	"sync"
	"testing"

	sqlite "github.com/ncruces/go-sqlite3/gormlite"
	gorm "gorm.io/gorm"
)

func setupQueueDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}

	if err := db.AutoMigrate(&Library{}, &Series{}, &Episode{}, &History{}, &Setting{}); err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	return db
}

func TestNewDBQueueWorkerRuns(t *testing.T) {
	db := setupQueueDB(t)
	q := NewDBQueue(db)
	defer q.Close()

	done := make(chan bool, 1)
	go func() {
		q.Write(func(tx *gorm.DB) error {
			return tx.Create(&Library{RootPath: "/test"}).Error
		})
		done <- true
	}()

	<-done

	var libs []Library
	if err := db.Find(&libs).Error; err != nil {
		t.Fatalf("failed to query libraries: %v", err)
	}
	if len(libs) != 1 {
		t.Errorf("expected 1 library, got %d", len(libs))
	}
}

func TestWriteCreateRecord(t *testing.T) {
	db := setupQueueDB(t)
	q := NewDBQueue(db)
	defer q.Close()

	err := q.Write(func(tx *gorm.DB) error {
		return tx.Create(&Series{ID: "100", Title: "Queue Test Series"}).Error
	})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var series Series
	if err := db.First(&series, "title = ?", "Queue Test Series").Error; err != nil {
		t.Fatalf("failed to find created series: %v", err)
	}
	if series.Title != "Queue Test Series" {
		t.Errorf("series title = %q, want %q", series.Title, "Queue Test Series")
	}
}

func TestWriteReturnsError(t *testing.T) {
	db := setupQueueDB(t)
	q := NewDBQueue(db)
	defer q.Close()

	err := q.Write(func(tx *gorm.DB) error {
		return tx.Create(&Series{ID: "100", Title: "Unique Series"}).Error
	})
	if err != nil {
		t.Fatalf("first write should succeed: %v", err)
	}

	err = q.Write(func(tx *gorm.DB) error {
		return tx.Create(&Setting{Key: "dup_key", Value: []byte(`"a"`)}).Error
	})
	if err != nil {
		t.Fatalf("first setting write should succeed: %v", err)
	}

	err = q.Write(func(tx *gorm.DB) error {
		return tx.Create(&Setting{Key: "dup_key", Value: []byte(`"b"`)}).Error
	})
	if err == nil {
		t.Fatal("expected error for duplicate key, got nil")
	}
}

func TestCloseNormal(t *testing.T) {
	db := setupQueueDB(t)
	q := NewDBQueue(db)

	err := q.Write(func(tx *gorm.DB) error {
		return tx.Create(&Library{RootPath: "/close_test"}).Error
	})
	if err != nil {
		t.Fatalf("write before close failed: %v", err)
	}

	if err := q.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestConcurrentWritesSerialExecution(t *testing.T) {
	db := setupQueueDB(t)
	q := NewDBQueue(db)
	defer q.Close()

	n := 50
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := q.Write(func(tx *gorm.DB) error {
				return tx.Create(&Series{ID: fmt.Sprintf("conc_%d", idx), Title: "Concurrent Series"}).Error
			})
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent write failed: %v", err)
	}

	var count int64
	if err := db.Model(&Series{}).Where("title = ?", "Concurrent Series").Count(&count).Error; err != nil {
		t.Fatalf("failed to count series: %v", err)
	}
	if count != int64(n) {
		t.Errorf("expected %d series, got %d", n, count)
	}
}

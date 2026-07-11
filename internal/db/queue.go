package db

import (
	"sync"

	"gorm.io/gorm"
)

type DBQueue struct {
	db      *gorm.DB
	writeCh chan func(*gorm.DB) error
	errCh   chan error
	wg      sync.WaitGroup
}

func NewDBQueue(db *gorm.DB) *DBQueue {
	q := &DBQueue{
		db:      db,
		writeCh: make(chan func(*gorm.DB) error, 100),
		errCh:   make(chan error),
	}
	q.wg.Add(1)
	go q.worker()
	return q
}

func (q *DBQueue) worker() {
	defer q.wg.Done()
	for fn := range q.writeCh {
		err := fn(q.db)
		q.errCh <- err
	}
}

func (q *DBQueue) Write(fn func(*gorm.DB) error) error {
	q.writeCh <- fn
	return <-q.errCh
}

func (q *DBQueue) Read(fn func(*gorm.DB) error) error {
	return fn(q.db)
}

func (q *DBQueue) Close() error {
	close(q.writeCh)
	q.wg.Wait()
	close(q.errCh)
	return nil
}

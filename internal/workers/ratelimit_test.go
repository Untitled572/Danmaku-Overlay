package workers

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucketBasic(t *testing.T) {
	tb := NewTokenBucket(100*time.Millisecond, 3)

	for i := 0; i < 3; i++ {
		if !tb.Allow() {
			t.Fatalf("Allow() returned false on call %d, expected true", i+1)
		}
	}

	if tb.Allow() {
		t.Fatal("Allow() returned true after consuming all tokens, expected false")
	}
}

func TestTokenBucketWait(t *testing.T) {
	t.Run("immediate when tokens available", func(t *testing.T) {
		tb := NewTokenBucket(time.Hour, 1)
		ctx := context.Background()

		start := time.Now()
		err := tb.Wait(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Wait() returned error: %v", err)
		}
		if elapsed > 50*time.Millisecond {
			t.Fatalf("Wait() took %v, expected near-instant", elapsed)
		}
	})

	t.Run("blocks then succeeds", func(t *testing.T) {
		tb := NewTokenBucket(100*time.Millisecond, 1)
		ctx := context.Background()

		tb.Allow()

		done := make(chan error, 1)
		go func() {
			done <- tb.Wait(ctx)
		}()

		select {
		case err := <-done:
			t.Fatalf("Wait() returned too fast: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Wait() returned error: %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Wait() did not return after token refill")
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		tb := NewTokenBucket(time.Hour, 1)
		ctx := context.Background()
		tb.Allow()

		ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := tb.Wait(ctx)
		if err != context.DeadlineExceeded {
			t.Fatalf("Wait() returned %v, expected context.DeadlineExceeded", err)
		}
	})
}

func TestTokenBucketRefill(t *testing.T) {
	tb := NewTokenBucket(50*time.Millisecond, 1)

	if !tb.Allow() {
		t.Fatal("first Allow() should return true")
	}

	if tb.Allow() {
		t.Fatal("second Allow() should return false")
	}

	time.Sleep(60 * time.Millisecond)

	if !tb.Allow() {
		t.Fatal("Allow() after refill should return true")
	}
}

func TestTokenBucketConcurrency(t *testing.T) {
	tb := NewTokenBucket(50*time.Millisecond, 10)
	ctx := context.Background()

	var wg sync.WaitGroup
	var successes atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := tb.Wait(ctx)
			if err == nil {
				successes.Add(1)
			}
		}()
	}

	wg.Wait()

	if int(successes.Load()) != 10 {
		t.Fatalf("expected 10 successes, got %d", successes.Load())
	}
}

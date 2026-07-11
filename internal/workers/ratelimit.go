package workers

import (
	"context"
	"sync"
	"time"
)

type TokenBucket struct {
	mu       sync.Mutex
	rate     time.Duration
	capacity int
	tokens   int
	last     time.Time
}

func NewTokenBucket(rate time.Duration, capacity int) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		capacity: capacity,
		tokens:   capacity,
		last:     time.Now(),
	}
}

func (b *TokenBucket) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b.mu.Lock()
		b.refill()

		if b.tokens > 0 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}

		nextToken := b.last.Add(b.rate)
		b.mu.Unlock()

		timer := time.NewTimer(time.Until(nextToken))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func (b *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.last)
	added := int(elapsed / b.rate)
	if added > 0 {
		b.tokens += added
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = b.last.Add(time.Duration(added) * b.rate)
	}
}

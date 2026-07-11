package workers

import (
	"sync"
	"testing"
)

func TestNewLRUCache(t *testing.T) {
	c := newLRUCache(10)
	if c == nil {
		t.Fatal("newLRUCache returned nil")
	}
	if c.max != 10 {
		t.Errorf("max = %d, want 10", c.max)
	}
	if len(c.items) != 0 {
		t.Errorf("items len = %d, want 0", len(c.items))
	}
}

func TestLRUSetAndGet(t *testing.T) {
	c := newLRUCache(3)
	c.Set("a", 1)
	v, ok := c.Get("a")
	if !ok {
		t.Fatal("Get returned false for existing key")
	}
	if v != 1 {
		t.Errorf("value = %d, want 1", v)
	}
}

func TestLRUGetNotFound(t *testing.T) {
	c := newLRUCache(3)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("Get returned true for nonexistent key")
	}
}

func TestLRUSetUpdate(t *testing.T) {
	c := newLRUCache(3)
	c.Set("a", 1)
	c.Set("a", 42)
	v, ok := c.Get("a")
	if !ok {
		t.Fatal("Get returned false after update")
	}
	if v != 42 {
		t.Errorf("value = %d, want 42", v)
	}
}

func TestLRUEviction(t *testing.T) {
	c := newLRUCache(3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4)

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}
	for _, key := range []string{"b", "c", "d"} {
		if _, ok := c.Get(key); !ok {
			t.Errorf("expected %q to be present", key)
		}
	}
}

func TestLRUGetMovesToFront(t *testing.T) {
	c := newLRUCache(3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Get("b")
	c.Set("d", 4)

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted (least recently used)")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("expected 'b' to be present (moved to front by Get)")
	}
	if _, ok := c.Get("d"); !ok {
		t.Error("expected 'd' to be present")
	}
}

func TestLRemove(t *testing.T) {
	c := newLRUCache(3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Remove("a")

	if _, ok := c.Get("a"); ok {
		t.Error("Get returned true after Remove")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("Get returned false for non-removed key")
	}

	c.Remove("nonexistent")
}

func TestLRUConcurrency(t *testing.T) {
	c := newLRUCache(1000)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%26))
			c.Set(key, uint64(n))
			c.Get(key)
			if n%2 == 0 {
				c.Remove(key)
			}
		}(i)
	}
	wg.Wait()
}

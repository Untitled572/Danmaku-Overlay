package workers

import "sync"

type lruCache struct {
	mu    sync.Mutex
	max   int
	items map[string]*cacheEntry
	head  *cacheEntry
	tail  *cacheEntry
}

type cacheEntry struct {
	key   string
	value uint64
	prev  *cacheEntry
	next  *cacheEntry
}

func newLRUCache(max int) *lruCache {
	return &lruCache{
		max:   max,
		items: make(map[string]*cacheEntry),
	}
}

func (c *lruCache) Get(key string) (uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		return 0, false
	}

	c.moveToFront(entry)
	return entry.value, true
}

func (c *lruCache) Set(key string, value uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.items[key]; ok {
		entry.value = value
		c.moveToFront(entry)
		return
	}

	entry := &cacheEntry{key: key, value: value}
	c.items[key] = entry
	c.pushFront(entry)

	if len(c.items) > c.max {
		c.evictTail()
	}
}

func (c *lruCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		return
	}

	delete(c.items, key)
	c.removeNode(entry)
}

func (c *lruCache) moveToFront(entry *cacheEntry) {
	if entry == c.head {
		return
	}
	c.removeNode(entry)
	c.pushFront(entry)
}

func (c *lruCache) pushFront(entry *cacheEntry) {
	entry.prev = nil
	entry.next = c.head
	if c.head != nil {
		c.head.prev = entry
	}
	c.head = entry
	if c.tail == nil {
		c.tail = entry
	}
}

func (c *lruCache) removeNode(entry *cacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		c.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		c.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

func (c *lruCache) evictTail() {
	if c.tail == nil {
		return
	}
	delete(c.items, c.tail.key)
	c.removeNode(c.tail)
}

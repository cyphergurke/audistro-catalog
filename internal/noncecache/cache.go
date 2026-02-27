package noncecache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type entry struct {
	expiresAt int64
	elem      *list.Element
}

// Cache stores recently seen nonces with TTL and bounded size.
type Cache struct {
	mu         sync.Mutex
	entries    map[string]entry
	insertionQ *list.List
	maxEntries int
}

func New(maxEntries int) *Cache {
	if maxEntries <= 0 {
		maxEntries = 100000
	}
	return &Cache{
		entries:    make(map[string]entry, maxEntries),
		insertionQ: list.New(),
		maxEntries: maxEntries,
	}
}

// AddIfNotExists inserts key if missing/expired and returns true when a valid key already exists.
func (c *Cache) AddIfNotExists(key string, ttl time.Duration, now time.Time) bool {
	nowUnix := now.Unix()
	expiresAt := now.Add(ttl).Unix()

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.entries[key]; ok {
		if existing.expiresAt > nowUnix {
			return true
		}
		c.removeLocked(key, existing)
	}

	elem := c.insertionQ.PushBack(key)
	c.entries[key] = entry{expiresAt: expiresAt, elem: elem}
	c.evictLocked(nowUnix)
	return false
}

func (c *Cache) SweepExpired(now time.Time) int {
	nowUnix := now.Unix()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sweepExpiredLocked(nowUnix)
}

func (c *Cache) StartJanitor(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.SweepExpired(time.Now())
			}
		}
	}()
}

func (c *Cache) evictLocked(nowUnix int64) {
	c.sweepExpiredLocked(nowUnix)
	for len(c.entries) > c.maxEntries {
		front := c.insertionQ.Front()
		if front == nil {
			return
		}
		key := front.Value.(string)
		if existing, ok := c.entries[key]; ok {
			c.removeLocked(key, existing)
		} else {
			c.insertionQ.Remove(front)
		}
	}
}

func (c *Cache) sweepExpiredLocked(nowUnix int64) int {
	removed := 0
	for elem := c.insertionQ.Front(); elem != nil; {
		next := elem.Next()
		key := elem.Value.(string)
		existing, ok := c.entries[key]
		if !ok {
			c.insertionQ.Remove(elem)
			elem = next
			continue
		}
		if existing.expiresAt <= nowUnix {
			c.removeLocked(key, existing)
			removed++
		}
		elem = next
	}
	return removed
}

func (c *Cache) removeLocked(key string, existing entry) {
	delete(c.entries, key)
	if existing.elem != nil {
		c.insertionQ.Remove(existing.elem)
	}
}

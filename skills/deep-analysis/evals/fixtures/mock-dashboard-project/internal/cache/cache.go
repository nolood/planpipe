package cache

import (
	"sync"
	"time"
)

// Cache is a simple in-memory TTL cache.
// Currently NOT used by the analytics service — it was added during
// an earlier optimization attempt but never wired in.
// TODO: Consider using this for caching ClickHouse query results.
type Cache struct {
	mu      sync.RWMutex
	items   map[string]cacheItem
	ttl     time.Duration
	maxSize int
}

type cacheItem struct {
	value     any
	expiresAt time.Time
}

func New(ttl time.Duration, maxSize int) *Cache {
	c := &Cache{
		items:   make(map[string]cacheItem),
		ttl:     ttl,
		maxSize: maxSize,
	}
	go c.cleanup()
	return c
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.maxSize {
		// Simple eviction: remove first expired item found
		for k, v := range c.items {
			if time.Now().After(v.expiresAt) {
				delete(c.items, k)
				break
			}
		}
	}

	c.items[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		for k, v := range c.items {
			if time.Now().After(v.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

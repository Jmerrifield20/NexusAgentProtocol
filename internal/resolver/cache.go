package resolver

import (
	"sync"
	"time"
)

// cacheEntry holds a cached resolution result.
type cacheEntry struct {
	endpoint   string
	status     string
	certSerial string
	expiresAt  time.Time
}

func (e *cacheEntry) expired() bool {
	return time.Now().After(e.expiresAt)
}

// resolverCache is a thread-safe in-memory LRU-style cache for resolved endpoints.
// Entries expire after a configurable TTL. A background goroutine evicts stale entries.
type resolverCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

func newResolverCache(ttl time.Duration) *resolverCache {
	rc := &resolverCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
	return rc
}

// get looks up a cached entry by agent URI key.
func (c *resolverCache) get(key string) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || e.expired() {
		return nil, false
	}
	return e, true
}

// set stores an entry in the cache.
func (c *resolverCache) set(key, endpoint, status, certSerial string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &cacheEntry{
		endpoint:   endpoint,
		status:     status,
		certSerial: certSerial,
		expiresAt:  time.Now().Add(c.ttl),
	}
}

// invalidate removes a specific entry from the cache.
func (c *resolverCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// evict removes all expired entries.
func (c *resolverCache) evict() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for k, e := range c.entries {
		if e.expired() {
			delete(c.entries, k)
			n++
		}
	}
	return n
}

// len returns the number of cached entries (including expired).
func (c *resolverCache) len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

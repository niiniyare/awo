// Package fakecache provides an in-memory implementation of cache.Cache and
// cache.Counter for use in tests.
package fakecache

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"awo.so/awo/cache"
)

// entry holds a cached value with optional expiry.
type entry struct {
	data    []byte
	expires time.Time // zero means no expiry
}

func (e *entry) expired() bool {
	return !e.expires.IsZero() && time.Now().After(e.expires)
}

// Cache is an in-memory implementation of cache.Cache and cache.Counter.
// It is safe for concurrent use.
type Cache struct {
	mu       sync.RWMutex
	entries  map[string]*entry
	counters map[string]int64
}

// New creates an empty Cache.
func New() *Cache {
	return &Cache{
		entries:  make(map[string]*entry),
		counters: make(map[string]int64),
	}
}

// Reset removes all cached entries.
func (c *Cache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*entry)
	c.counters = make(map[string]int64)
}

// Get retrieves the value for key into dst.
// dst must be a non-nil pointer. Returns cache.ErrMiss if key is absent or expired.
func (c *Cache) Get(_ context.Context, key string, dst any) error {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || e.expired() {
		return cache.ErrMiss
	}

	return json.Unmarshal(e.data, dst)
}

// Set stores value under key with the given TTL.
// A zero TTL stores without expiry.
func (c *Cache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	e := &entry{data: data}
	if ttl > 0 {
		e.expires = time.Now().Add(ttl)
	}

	c.mu.Lock()
	c.entries[key] = e
	c.mu.Unlock()
	return nil
}

// Delete removes the key. No error if key does not exist.
func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
	return nil
}

// DeletePrefix removes all keys with the given prefix.
func (c *Cache) DeletePrefix(_ context.Context, prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
	return nil
}

// Exists reports whether key is present and not expired.
func (c *Cache) Exists(_ context.Context, key string) (bool, error) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || e.expired() {
		return false, nil
	}
	return true, nil
}

// Increment atomically increments key by delta and returns the new value.
func (c *Cache) Increment(_ context.Context, key string, delta int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters[key] += delta
	return c.counters[key], nil
}

// IncrementWithReset increments key by delta. If the key didn't exist, it
// sets a conceptual expiry (simulated by tracking the window start). The
// fake implementation ignores the window for simplicity — it always increments.
func (c *Cache) IncrementWithReset(_ context.Context, key string, delta int64, _ time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters[key] += delta
	return c.counters[key], nil
}

// Keys returns all non-expired keys currently in the cache (for testing assertions).
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []string
	for k, e := range c.entries {
		if !e.expired() {
			out = append(out, k)
		}
	}
	return out
}

// GetCounter returns the current counter value for key.
func (c *Cache) GetCounter(key string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.counters[key]
}

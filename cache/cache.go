// Package cache defines the cache abstraction used by platform modules.
//
// The framework uses caching for: SDUI page schemas, feature flag evaluations,
// Casbin policy results, and naming series counters. All caches are tenant-
// scoped by convention — cache keys should embed tenant ID to prevent cross-
// tenant data leakage.
//
// Concrete implementations: awo/contrib/redis (production),
// awo/contrib/memcache (in-process, for tests).
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss is returned when a key is not found in the cache.
var ErrMiss = errors.New("cache: miss")

// Cache provides get/set/delete operations over a key-value store.
// All methods must be safe for concurrent use.
type Cache interface {
	// Get retrieves the value for key into dst.
	// dst must be a non-nil pointer. Returns ErrMiss if key is absent.
	Get(ctx context.Context, key string, dst any) error

	// Set stores value under key with the given TTL.
	// A zero TTL stores without expiry (implementation-dependent).
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes the key. No error if key does not exist.
	Delete(ctx context.Context, key string) error

	// DeletePrefix removes all keys with the given prefix.
	// Used to invalidate groups of related cache entries.
	DeletePrefix(ctx context.Context, prefix string) error

	// Exists reports whether key is present in the cache.
	Exists(ctx context.Context, key string) (bool, error)
}

// Counter provides atomic increment operations for naming series and
// rate limiting. Implemented separately from Cache because Redis INCR
// semantics differ from GET/SET.
type Counter interface {
	// Increment atomically increments key by delta and returns the new value.
	// If key does not exist, it is created with value delta.
	Increment(ctx context.Context, key string, delta int64) (int64, error)

	// IncrementWithReset increments key, resetting it to delta when the TTL
	// expires. Used for rate-limit windows.
	IncrementWithReset(ctx context.Context, key string, delta int64, window time.Duration) (int64, error)
}

// IsMiss reports whether err is a cache miss.
func IsMiss(err error) bool {
	return errors.Is(err, ErrMiss)
}

// NoopCache discards all writes and reports misses on all reads.
// Useful in tests where caching is not under test.
type NoopCache struct{}

func (NoopCache) Get(_ context.Context, _ string, _ any) error                  { return ErrMiss }
func (NoopCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error { return nil }
func (NoopCache) Delete(_ context.Context, _ string) error                      { return nil }
func (NoopCache) DeletePrefix(_ context.Context, _ string) error                { return nil }
func (NoopCache) Exists(_ context.Context, _ string) (bool, error)              { return false, nil }

// NoopCounter discards all increments and always returns 0.
// Useful when Redis is unavailable and rate limiting should be disabled.
type NoopCounter struct{}

func (NoopCounter) Increment(_ context.Context, _ string, _ int64) (int64, error) {
	return 0, nil
}

func (NoopCounter) IncrementWithReset(_ context.Context, _ string, _ int64, _ time.Duration) (int64, error) {
	return 0, nil
}

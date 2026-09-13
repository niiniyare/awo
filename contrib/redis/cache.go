// Package redis provides the Redis implementation of awo/cache.Cache and
// awo/cache.Counter using go-redis v8.
//
// Key serialization: values are JSON-encoded before storage. The encoding
// round-trip preserves all JSON-representable types. Non-JSON types (e.g.
// custom structs) must be registered as JSON before storage.
//
// All operations respect ctx cancellation and deadline. Redis errors are
// returned unwrapped — callers should not depend on their concrete type.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"awo.so/awo/cache"
)

// Client wraps go-redis and implements cache.Cache and cache.Counter.
type Client struct {
	rdb *goredis.Client
}

// New creates a Client from an existing go-redis client.
func New(rdb *goredis.Client) *Client {
	return &Client{rdb: rdb}
}

// Ensure compile-time interface satisfaction.
var (
	_ cache.Cache   = (*Client)(nil)
	_ cache.Counter = (*Client)(nil)
)

// Get retrieves the value for key into dst (JSON-decoded).
// Returns cache.ErrMiss if the key does not exist.
func (c *Client) Get(ctx context.Context, key string, dst any) error {
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return cache.ErrMiss
		}
		return fmt.Errorf("redis.Get %q: %w", key, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("redis.Get %q: unmarshal: %w", key, err)
	}
	return nil
}

// Set stores value (JSON-encoded) under key with ttl.
// A zero ttl stores without expiry.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis.Set %q: marshal: %w", key, err)
	}
	if err := c.rdb.Set(ctx, key, raw, ttl).Err(); err != nil {
		return fmt.Errorf("redis.Set %q: %w", key, err)
	}
	return nil
}

// Delete removes the key. No error if the key does not exist.
func (c *Client) Delete(ctx context.Context, key string) error {
	if err := c.rdb.Del(ctx, key).Err(); err != nil && !errors.Is(err, goredis.Nil) {
		return fmt.Errorf("redis.Delete %q: %w", key, err)
	}
	return nil
}

// DeletePrefix removes all keys with the given prefix using SCAN + DEL.
// This is O(N) in the number of matching keys; use sparingly.
func (c *Client) DeletePrefix(ctx context.Context, prefix string) error {
	iter := c.rdb.Scan(ctx, 0, prefix+"*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("redis.DeletePrefix %q: scan: %w", prefix, err)
	}
	if len(keys) == 0 {
		return nil
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis.DeletePrefix %q: del: %w", prefix, err)
	}
	return nil
}

// Exists reports whether key is present.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis.Exists %q: %w", key, err)
	}
	return n > 0, nil
}

// Increment atomically increments key by delta and returns the new value.
func (c *Client) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	val, err := c.rdb.IncrBy(ctx, key, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("redis.Increment %q: %w", key, err)
	}
	return val, nil
}

// IncrementWithReset increments key by delta; sets TTL to window if the key
// was just created (i.e. value equals delta after increment).
func (c *Client) IncrementWithReset(ctx context.Context, key string, delta int64, window time.Duration) (int64, error) {
	pipe := c.rdb.TxPipeline()
	incrCmd := pipe.IncrBy(ctx, key, delta)
	// Always refresh TTL — sliding window.
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("redis.IncrementWithReset %q: %w", key, err)
	}
	return incrCmd.Val(), nil
}

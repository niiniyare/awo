// Package cache implements the three-level SDUI cache.
//
// Level 1: In-process CompiledSchema cache (not in this package — owned by the compiler).
// Level 2: Redis widget tree cache (serialised JSON of widget.Node tree).
// Level 3: Redis rendered output cache (serialised JSON of RenderedOutput).
//
// Cache key structure (all levels):
//
//	sdui:v1:{level}:{entity}:{view}:{renderer_id}:{renderer_ver}:{locale}:{tenant_id_hash}:{schema_fp}:{perm_fp}
//
// All key components are hex/base64 encoded. No raw UUIDs or user data.
//
// TTL policy:
//   - Level 2 (widget tree): 5 minutes. Invalidated by schema_fp or perm_fp change.
//   - Level 3 (rendered output): 10 minutes. Invalidated by schema_fp, perm_fp, or renderer_ver change.
//
// Race condition handling:
//   - singleflight.Group prevents thundering herd on Level 2 and Level 3 misses.
//   - Multiple concurrent requests for the same cache key share a single generation/render call.
//
// Redis dependency: this package requires a redis client. It accepts a RedisClient
// interface to avoid a hard dependency on go-redis. Pass nil to use the NoopCache.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// keyVersion is the global cache key prefix version. Increment to
	// invalidate all SDUI cache entries across all Redis instances.
	keyVersion = "v1"

	// ttlWidgetTree is the Level 2 (widget tree JSON) TTL.
	ttlWidgetTree = 5 * time.Minute

	// ttlRenderedOutput is the Level 3 (rendered output JSON) TTL.
	ttlRenderedOutput = 10 * time.Minute
)

// RedisClient is the minimal Redis interface required by this package.
// The concrete implementation is typically *redis.Client from go-redis/v8.
type RedisClient interface {
	// Get retrieves a value. Returns ("", ErrCacheMiss) when key is not found.
	Get(ctx context.Context, key string) (string, error)
	// Set stores a value with an expiration duration.
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// ErrCacheMiss is returned by RedisClient.Get when the key is not found.
// Cache implementations must return exactly this error (or a wrapping thereof)
// for misses so that the cache layer can distinguish misses from errors.
var ErrCacheMiss = fmt.Errorf("sdui/cache: miss")

// KeyParams holds all dimensions required to construct a deterministic cache key.
// Every field contributes to the key — missing any field risks cache collisions.
type KeyParams struct {
	Level           string // "l2" or "l3"
	EntityName      string
	ViewMode        string
	RendererID      string
	RendererVersion string
	Locale          string
	TenantIDHash    string // SHA-256 of tenant UUID — not the raw UUID
	SchemaFP        string // compiled schema fingerprint
	PermFP          string // permission fingerprint from PolicyEvaluator
	// FeatureFlagFP is a fingerprint of the feature flags active for this
	// viewer+tenant. Empty string means feature flags are not in use;
	// schemas are flag-invariant and no extra cache dimension is added.
	FeatureFlagFP string
}

// Key returns the Redis cache key for the given parameters.
// The key is safe for use as a Redis key — no spaces, no special characters
// beyond colons and hex characters.
func Key(p KeyParams) string {
	parts := []string{
		"sdui",
		keyVersion,
		p.Level,
		safeComponent(p.EntityName),
		safeComponent(p.ViewMode),
		safeComponent(p.RendererID),
		safeComponent(p.RendererVersion),
		safeComponent(p.Locale),
		p.TenantIDHash,
		p.SchemaFP,
		p.PermFP,
		p.FeatureFlagFP, // empty string adds no cache dimension when flags unused
	}
	return strings.Join(parts, ":")
}

// HashTenantID returns the hex SHA-256 of a tenant ID string.
// Use this to convert a UUID string to the TenantIDHash key component.
func HashTenantID(tenantID string) string {
	h := sha256.Sum256([]byte(tenantID))
	return hex.EncodeToString(h[:])
}

// safeComponent replaces characters that are not safe in Redis keys with underscores.
func safeComponent(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '.' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Cache provides Level 2 and Level 3 SDUI caching backed by Redis.
// When the RedisClient is nil, all operations are no-ops (NoopCache behaviour).
type Cache struct {
	redis   RedisClient
	l2group singleflight.Group
	l3group singleflight.Group
}

// New returns a Cache backed by the given RedisClient.
// Pass nil to get a no-op cache (useful in tests and development).
func New(client RedisClient) *Cache {
	return &Cache{redis: client}
}

// GetWidgetTree retrieves a cached widget tree JSON from Level 2.
// Returns (nil, nil) on a cache miss. Returns (nil, err) on a Redis error.
func (c *Cache) GetWidgetTree(ctx context.Context, key string) ([]byte, error) {
	if c.redis == nil {
		return nil, nil
	}
	val, err := c.redis.Get(ctx, key)
	if err != nil {
		if isErrMiss(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sdui/cache: L2 get %q: %w", key, err)
	}
	return []byte(val), nil
}

// SetWidgetTree stores a widget tree JSON in Level 2 with the standard TTL.
// Errors are returned but treated as non-fatal by the caller — a failed write
// is a missed cache opportunity, not an application error.
func (c *Cache) SetWidgetTree(ctx context.Context, key string, data []byte) error {
	if c.redis == nil {
		return nil
	}
	if err := c.redis.Set(ctx, key, string(data), ttlWidgetTree); err != nil {
		return fmt.Errorf("sdui/cache: L2 set %q: %w", key, err)
	}
	return nil
}

// GetRenderedOutput retrieves cached rendered output JSON from Level 3.
// Returns (nil, nil) on a cache miss. Returns (nil, err) on a Redis error.
func (c *Cache) GetRenderedOutput(ctx context.Context, key string) ([]byte, error) {
	if c.redis == nil {
		return nil, nil
	}
	val, err := c.redis.Get(ctx, key)
	if err != nil {
		if isErrMiss(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sdui/cache: L3 get %q: %w", key, err)
	}
	return []byte(val), nil
}

// SetRenderedOutput stores rendered output JSON in Level 3 with the standard TTL.
func (c *Cache) SetRenderedOutput(ctx context.Context, key string, data []byte) error {
	if c.redis == nil {
		return nil
	}
	if err := c.redis.Set(ctx, key, string(data), ttlRenderedOutput); err != nil {
		return fmt.Errorf("sdui/cache: L3 set %q: %w", key, err)
	}
	return nil
}

// DoWidgetTree executes fn under singleflight for the given key (Level 2).
// If multiple concurrent callers request the same key, only one fn is executed;
// the result is shared among all callers.
// fn should return ([]byte, error) where []byte is the serialised widget tree.
func (c *Cache) DoWidgetTree(key string, fn func() ([]byte, error)) ([]byte, error) {
	result, err, _ := c.l2group.Do(key, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	return result.([]byte), nil
}

// DoRenderedOutput executes fn under singleflight for the given key (Level 3).
func (c *Cache) DoRenderedOutput(key string, fn func() ([]byte, error)) ([]byte, error) {
	result, err, _ := c.l3group.Do(key, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	return result.([]byte), nil
}

// isErrMiss reports whether err represents a cache miss.
func isErrMiss(err error) bool {
	if err == nil {
		return false
	}
	return err == ErrCacheMiss || strings.Contains(err.Error(), "nil") || strings.Contains(err.Error(), "redis: nil")
}

// MarshalJSON is a convenience wrapper for encoding/json.Marshal with a descriptive error.
func MarshalJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("sdui/cache: marshal: %w", err)
	}
	return data, nil
}

// UnmarshalJSON is a convenience wrapper for encoding/json.Unmarshal with a descriptive error.
func UnmarshalJSON(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("sdui/cache: unmarshal: %w", err)
	}
	return nil
}

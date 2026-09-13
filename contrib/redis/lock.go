package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"awo.so/awo/lock"
)

// Locker implements lock.Locker using Redis SET NX (single-instance Redlock).
// For multi-node Redis, upgrade to the full Redlock algorithm.
type Locker struct {
	rdb *goredis.Client
}

// NewLocker creates a Redis-backed distributed locker.
func NewLocker(rdb *goredis.Client) *Locker {
	return &Locker{rdb: rdb}
}

var _ lock.Locker = (*Locker)(nil)

// Acquire blocks until the lock is acquired or ctx is cancelled.
func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (lock.Lock, error) {
	token, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("redis.lock.Acquire: %w", err)
	}
	for {
		ok, err := l.rdb.SetNX(ctx, lockKey(key), token, ttl).Result()
		if err != nil {
			return nil, fmt.Errorf("redis.lock.Acquire %q: %w", key, err)
		}
		if ok {
			return &redisLock{rdb: l.rdb, key: lockKey(key), token: token}, nil
		}
		select {
		case <-ctx.Done():
			return nil, lock.ErrLockNotAcquired
		case <-time.After(50 * time.Millisecond):
			// Retry with small backoff.
		}
	}
}

// TryAcquire attempts to acquire the lock without blocking.
func (l *Locker) TryAcquire(ctx context.Context, key string, ttl time.Duration) (lock.Lock, error) {
	token, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("redis.lock.TryAcquire: %w", err)
	}
	ok, err := l.rdb.SetNX(ctx, lockKey(key), token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("redis.lock.TryAcquire %q: %w", key, err)
	}
	if !ok {
		return nil, lock.ErrLockNotAcquired
	}
	return &redisLock{rdb: l.rdb, key: lockKey(key), token: token}, nil
}

// redisLock is an acquired Redis lock.
type redisLock struct {
	rdb   *goredis.Client
	key   string
	token string
}

// Release releases the lock using a Lua script to ensure atomicity.
// The script only deletes the key if the stored token matches our token.
func (rl *redisLock) Release(ctx context.Context) error {
	script := `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`
	n, err := rl.rdb.Eval(ctx, script, []string{rl.key}, rl.token).Int64()
	if err != nil {
		return fmt.Errorf("redis.lock.Release %q: %w", rl.key, err)
	}
	if n == 0 {
		return fmt.Errorf("redis.lock.Release %q: lock expired or was released by another owner", rl.key)
	}
	return nil
}

// Extend extends the lock TTL.
func (rl *redisLock) Extend(ctx context.Context, ttl time.Duration) error {
	script := `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end`
	ms := ttl.Milliseconds()
	n, err := rl.rdb.Eval(ctx, script, []string{rl.key}, rl.token, ms).Int64()
	if err != nil {
		return fmt.Errorf("redis.lock.Extend %q: %w", rl.key, err)
	}
	if n == 0 {
		return fmt.Errorf("redis.lock.Extend %q: lock expired or owned by another", rl.key)
	}
	return nil
}

func lockKey(key string) string {
	return "lock:" + key
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

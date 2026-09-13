// Package lock defines the distributed lock abstraction.
//
// Used by: naming series (atomic sequence generation), module activation
// (prevent concurrent schema mutations), cache stampede prevention.
//
// Concrete implementation: awo/contrib/redis (Redlock algorithm).
// Test implementation: awo/contrib/memlock (in-process mutex, non-distributed).
package lock

import (
	"context"
	"errors"
	"time"
)

// ErrLockNotAcquired is returned when a lock cannot be acquired within the
// deadline or because it is held by another owner.
var ErrLockNotAcquired = errors.New("lock: not acquired")

// Lock represents an acquired distributed lock. The caller must call Release
// when finished; failing to do so leaves the lock held until its TTL expires.
type Lock interface {
	// Release releases the lock. Returns an error if the lock has already
	// expired or was released by another caller.
	Release(ctx context.Context) error

	// Extend extends the lock TTL by ttl from now. Used for long-running
	// operations that must hold the lock longer than initially requested.
	Extend(ctx context.Context, ttl time.Duration) error
}

// Locker acquires distributed locks.
type Locker interface {
	// Acquire attempts to acquire a lock on key with the given TTL.
	// Blocks until the lock is acquired or ctx is cancelled.
	// Returns ErrLockNotAcquired if ctx expires before acquisition.
	Acquire(ctx context.Context, key string, ttl time.Duration) (Lock, error)

	// TryAcquire attempts to acquire the lock without blocking.
	// Returns ErrLockNotAcquired immediately if the lock is held.
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

// IsNotAcquired reports whether err indicates a lock acquisition failure.
func IsNotAcquired(err error) bool {
	return errors.Is(err, ErrLockNotAcquired)
}

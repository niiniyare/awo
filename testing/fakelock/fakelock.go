// Package fakelock provides an in-memory implementation of lock.Locker and
// lock.Lock for use in tests.
//
// The implementation uses a sync.Mutex map keyed by lock key. It is not
// distributed — it is only safe for tests within a single process.
package fakelock

import (
	"context"
	"sync"
	"time"

	"awo.so/awo/lock"
)

// Locker is an in-memory implementation of lock.Locker.
type Locker struct {
	mu    sync.Mutex
	locks map[string]bool
}

// New creates an empty Locker.
func New() *Locker {
	return &Locker{
		locks: make(map[string]bool),
	}
}

// TryAcquire attempts to acquire the lock without blocking.
// Returns lock.ErrLockNotAcquired immediately if the lock is held.
func (l *Locker) TryAcquire(_ context.Context, key string, _ time.Duration) (lock.Lock, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locks[key] {
		return nil, lock.ErrLockNotAcquired
	}
	l.locks[key] = true
	return &fakeLock{locker: l, key: key}, nil
}

// Acquire attempts to acquire a lock on key with the given TTL.
// Polls until the lock is acquired or ctx is cancelled.
func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (lock.Lock, error) {
	for {
		lk, err := l.TryAcquire(ctx, key, ttl)
		if err == nil {
			return lk, nil
		}

		select {
		case <-ctx.Done():
			return nil, lock.ErrLockNotAcquired
		case <-time.After(5 * time.Millisecond):
			// retry
		}
	}
}

// Release releases a held lock.
func (l *Locker) release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.locks, key)
}

// IsHeld reports whether the given key is currently locked (for testing).
func (l *Locker) IsHeld(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locks[key]
}

// Reset releases all locks.
func (l *Locker) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.locks = make(map[string]bool)
}

// fakeLock implements lock.Lock.
type fakeLock struct {
	mu       sync.Once
	locker   *Locker
	key      string
	released bool
}

// Release releases the lock.
func (fl *fakeLock) Release(_ context.Context) error {
	fl.mu.Do(func() {
		fl.locker.release(fl.key)
		fl.released = true
	})
	return nil
}

// Extend is a no-op in the fake implementation.
func (fl *fakeLock) Extend(_ context.Context, _ time.Duration) error {
	return nil
}

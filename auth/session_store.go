// Package auth — session_store.go
//
// SessionStore is the persistence interface for short-lived human sessions.
// It is defined here (in awo/auth) so that platform/iam can depend on the
// abstraction without importing any infrastructure library. The production
// implementation lives in awo/contrib/redis.
package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrSessionNotFound is returned by [SessionStore.Load] when no session exists
// for the given token. This covers three cases:
//   - the token was never issued
//   - the session has expired (TTL elapsed and the store evicted it)
//   - the session was explicitly revoked via [SessionStore.Delete] or [SessionStore.DeleteAll]
//
// ErrSessionNotFound is semantically distinct from a generic cache miss
// (cache.ErrMiss). IAM maps it to HTTP 401; a generic cache miss from another
// subsystem has different handling. awo/auth MUST NOT import awo/cache — this
// sentinel lives here so that iam can depend only on awo/auth.
var ErrSessionNotFound = errors.New("auth: session not found")

// SessionStore is the persistence interface for short-lived human sessions.
//
// Implementations enforce TTL: a session whose ExpiresAt has elapsed MUST NOT
// be returned by Load — it must appear as absent (returning ErrSessionNotFound).
//
// Implementations must be safe for concurrent use.
//
// The framework provides:
//   - [awo/contrib/redis.RedisSessionStore] — production Redis-backed implementation
//   - MemSessionStore (in test packages) — in-memory test double
type SessionStore interface {
	// Store persists a new session with TTL matching session.ExpiresAt.
	//
	// Store is called on the critical path during login. If Store returns an
	// error, login fails — sessions cannot be issued when the store is
	// unavailable. This is correct security behaviour.
	//
	// Additionally, Store records the session token in a per-user index to
	// enable bulk revocation via [DeleteAll]. This index write is best-effort:
	// failure is logged but does not cause Store to return an error.
	Store(ctx context.Context, session *Session) error

	// Load retrieves the session for the given token.
	//
	// Returns [ErrSessionNotFound] when the token is absent or has expired.
	// IAM callers must map this to HTTP 401.
	//
	// Returns any other error when the store is unavailable. IAM callers must
	// map non-ErrSessionNotFound errors to HTTP 503 — not 401 — to distinguish
	// infrastructure failures from genuine authentication failures.
	Load(ctx context.Context, token string) (*Session, error)

	// Delete removes the session identified by token and records a revocation
	// tombstone for it (see IsRevoked) valid for the remainder of the session's
	// original lifetime.
	//
	// Returns an error if either the primary session removal or the tombstone
	// write fails. The per-user index entry for this token is removed as a
	// best-effort side effect; index removal failure is logged but does not
	// cause Delete to return an error.
	//
	// Callers must treat Delete errors as hard failures: the session may still
	// be live (or resurrectable via a fallback recovery path) if the primary
	// removal or the tombstone write failed.
	Delete(ctx context.Context, session *Session) error

	// ListUserTokens returns all non-expired token strings currently held for
	// the given user within the tenant.
	//
	// Used by bulk revocation (role changes, forced logout) to enumerate
	// sessions before calling DeleteAll. Returns nil, nil when the user has
	// no active sessions.
	ListUserTokens(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)

	// DeleteAll removes every session in tokens from the store, records a
	// revocation tombstone for each (see IsRevoked), and clears the user's
	// session index for the given (tenantID, userID) pair.
	//
	// Called after a successful ListUserTokens during role-change revocation
	// and admin forced-logout. Returns an error if the bulk removal or the
	// tombstone writes fail. Index cleanup is best-effort; its failure does
	// not cause DeleteAll to return an error.
	DeleteAll(ctx context.Context, tenantID, userID uuid.UUID, tokens []string) error

	// IsRevoked reports whether token has an active revocation tombstone,
	// written by a prior Delete or DeleteAll call.
	//
	// This is the authoritative defense against session resurrection: a
	// durable-store recovery path (e.g. a PostgreSQL fallback used when this
	// store has no record of the token at all — evicted, restarted, or never
	// synced) MUST check IsRevoked before trusting any record it finds there.
	// The durable store's own revocation write is typically best-effort and
	// can fail independently of Delete/DeleteAll succeeding here; the
	// tombstone is the single source of truth that cannot silently disagree
	// with it. Once Delete/DeleteAll has returned successfully for a token,
	// IsRevoked(token) MUST return true for at least the remainder of that
	// token's original validity window, regardless of the durable store's
	// state.
	//
	// Returns a non-nil error only for infrastructure failures (the store is
	// unavailable). Callers must NOT treat an error as "not revoked" — an
	// unreachable store means the revocation status is unknown, and callers
	// must fail closed (treat the caller-facing operation as unavailable, not
	// as authenticated).
	IsRevoked(ctx context.Context, token string) (bool, error)
}

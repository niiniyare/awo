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

	// Delete removes the session identified by token.
	//
	// Returns an error only if the primary session removal fails. The per-user
	// index entry for this token is removed as a best-effort side effect; index
	// removal failure is logged but does not cause Delete to return an error.
	//
	// Callers must treat Delete errors as hard failures: the session may still
	// be live if the primary removal failed.
	Delete(ctx context.Context, session *Session) error

	// ListUserTokens returns all non-expired token strings currently held for
	// the given user within the tenant.
	//
	// Used by bulk revocation (role changes, forced logout) to enumerate
	// sessions before calling DeleteAll. Returns nil, nil when the user has
	// no active sessions.
	ListUserTokens(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)

	// DeleteAll removes every session in tokens from the store and clears the
	// user's session index for the given (tenantID, userID) pair.
	//
	// Called after a successful ListUserTokens during role-change revocation
	// and admin forced-logout. Returns an error if the bulk removal fails.
	// Index cleanup is best-effort; its failure does not cause DeleteAll to
	// return an error.
	DeleteAll(ctx context.Context, tenantID, userID uuid.UUID, tokens []string) error
}

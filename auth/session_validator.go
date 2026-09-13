package auth

import "context"

// SessionValidator is the interface the API authentication middleware uses to
// validate an incoming Bearer token or API token header.
//
// Implementations may use Redis, PostgreSQL, in-memory state, or any
// combination. The framework middleware never imports a concrete IAM
// implementation — it depends only on this interface.
//
// The default implementation is [awo.so/awo/platform/iam.AuthService], which
// validates human sessions from Redis and API tokens from cache+DB.
//
// Callers MUST NOT map infrastructure errors to 401. A returned non-nil error
// that is not [ErrSessionNotFound] indicates an infrastructure failure and
// MUST produce a 500 response.
type SessionValidator interface {
	// ValidateToken checks a Bearer session token. Returns the associated
	// Session on success. Returns [ErrSessionNotFound] when the token is
	// unknown, expired, or revoked. Returns a wrapped infrastructure error
	// for Redis/PostgreSQL failures.
	ValidateToken(ctx context.Context, token string) (*Session, error)

	// ValidateAPIToken checks a raw API token string (not a hash). The
	// implementation is responsible for hashing and looking up the token.
	// Returns the associated service-account Session on success.
	// Returns [ErrSessionNotFound] when the token is unknown, expired, or revoked.
	ValidateAPIToken(ctx context.Context, rawToken string) (*Session, error)
}

package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"awo.so/awo/def"
	"github.com/google/uuid"
)

// Session is the complete authentication state for one authenticated principal.
// Sessions are stored in Redis as JSON-serialized values under the key
// "session:{token}". The middleware reads and validates the session on every
// authenticated request.
//
// Session is constructed at login and stored in Redis. It is NOT stored in
// PostgreSQL. The Redis TTL is set to ExpiresAt − now at session creation.
//
// Session is a frozen struct (ADR-004). Its fields MUST NOT be changed without
// a new ADR and a breaking-change notice.
type Session struct {
	// Token is the opaque session token and the Redis key suffix.
	// Generated as a cryptographically random 256-bit value, base64url-encoded.
	// MUST NOT appear in application logs or error messages.
	Token string `json:"token"`

	// UserID is the UUID of the authenticated human user.
	// uuid.Nil for service account sessions.
	UserID uuid.UUID `json:"user_id"`

	// ServiceAccountID is the UUID of the authenticated service account.
	// uuid.Nil for human user sessions.
	ServiceAccountID uuid.UUID `json:"service_account_id"`

	// TenantID is the UUID of the tenant this session operates within.
	TenantID uuid.UUID `json:"tenant_id"`

	// Roles is the complete set of role names at the time of login.
	// Role changes take effect at next login; force revocation by calling
	// DEL session:{token} in Redis.
	Roles []string `json:"roles"`

	// ExpiresAt is the wall-clock time when this session expires.
	// The middleware MUST reject sessions where time.Now().After(ExpiresAt).
	ExpiresAt time.Time `json:"expires_at"`

	// IssuedAt is the wall-clock time when this session was created.
	IssuedAt time.Time `json:"issued_at"`

	// DeviceID is an optional client device identifier for session tracking.
	DeviceID string `json:"device_id,omitempty"`

	// IPAddress is the client IP at login time. Used for security alerting.
	IPAddress string `json:"ip_address,omitempty"`

	// RequestID is set by the middleware on each request for log correlation.
	// This field MUST NOT be stored in Redis — it is ephemeral per request.
	RequestID string `json:"-"`

	// Metadata holds extensible per-session data stored as JSONB in iam_sessions
	// and as JSON in Redis. Callers may read and write arbitrary keys.
	//
	// The framework reserves all keys prefixed with "awo:".
	// Module authors may use their module name as a key prefix (e.g. "finance:").
	//
	// NEVER store secrets, passwords, or tokens in Metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// IsExpired returns true when the session has passed its expiry time.
// The middleware calls this as a defense-in-depth check even though Redis TTL
// would expire the key automatically.
func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// ToActor converts the session to a [def.Actor] suitable for injection into
// hooks and action handlers via the request context.
func (s *Session) ToActor() *def.Actor {
	return &def.Actor{
		UserID:           s.UserID,
		ServiceAccountID: s.ServiceAccountID,
		TenantID:         s.TenantID,
		Roles:            append([]string(nil), s.Roles...),
	}
}

// ToViewer converts the session to a [ViewerContext] for embedding in
// context.Context via [WithViewer]. Called by the session validation middleware
// after all validation steps pass.
func (s *Session) ToViewer() ViewerContext {
	return NewViewer(s)
}

// TTL returns the duration until this session expires. Returns a negative
// duration when the session has already expired. Used to set the Redis TTL
// at session creation.
func (s *Session) TTL(now time.Time) time.Duration {
	return s.ExpiresAt.Sub(now)
}

// GenerateToken generates a cryptographically random session token.
// The token is 256 bits of entropy, base64url-encoded without padding.
// Call this when creating a new session at login time.
func GenerateToken() (string, error) {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

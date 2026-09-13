package audit

import "context"

// RequestContext carries per-request audit correlation values derived from the
// HTTP middleware stack. It is injected into context.Context by RequireAuth
// after session validation, and consumed by the pipeline's RunAuditRecord to
// populate AuditRecord.RequestID, AuditRecord.IPAddress, and AuditRecord.SessionID.
//
// SessionID is an HMAC-SHA256 digest of the raw session token — it uniquely
// correlates audit rows to a session without exposing the credential. The raw
// token never reaches the audit log.
type RequestContext struct {
	// RequestID is the X-Request-ID header value set by the RequestID middleware.
	RequestID string

	// IPAddress is the client IP as reported by Fiber (c.IP()).
	IPAddress string

	// SessionID is HMAC-SHA256(raw_session_token, AUDIT_SIGNING_SECRET).
	// Empty when auditSigningSecret is not configured or token is absent.
	SessionID string
}

// requestContextKey is the unexported context key type for RequestContext.
type requestContextKey struct{}

// WithRequestContext embeds rc into ctx. Called by the RequireAuth middleware
// once per authenticated request.
func WithRequestContext(ctx context.Context, rc RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey{}, rc)
}

// RequestContextFromContext extracts the RequestContext from ctx.
// Returns the zero value and false when RequireAuth has not run (e.g. system
// background operations, unauthenticated paths, or tests).
func RequestContextFromContext(ctx context.Context) (RequestContext, bool) {
	rc, ok := ctx.Value(requestContextKey{}).(RequestContext)
	return rc, ok
}

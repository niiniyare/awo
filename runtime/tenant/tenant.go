// Package tenant implements TenantContext propagation through context.Context.
//
// Every inbound request carries exactly one TenantContext, set by the Tenant
// Resolution middleware before any application logic executes. Background
// operations (Temporal activities, scheduled jobs) must construct a
// TenantContext explicitly and attach it before performing repository
// operations.
package tenant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// TenantContext carries the resolved tenant identity for a single request.
// It is the only way tenant information propagates through the system.
//
// Propagation rules (from TEN-001):
//   - MUST be passed via context.Context only
//   - MUST NOT be stored in global variables
//   - MUST NOT be passed as function parameters alongside context.Context
type TenantContext struct {
	// TenantID is the primary identifier. Always a valid, non-nil UUID.
	TenantID uuid.UUID

	// TenantSlug is the subdomain identifier (e.g. "acme").
	TenantSlug string

	// Locale is the BCP-47 locale tag for this tenant (e.g. "en-KE", "sw-KE").
	Locale string

	// Timezone is the IANA timezone name (e.g. "Africa/Nairobi").
	Timezone string

	// Currency is the ISO 4217 currency code (e.g. "KES").
	Currency string
}

type contextKey struct{}

// WithContext attaches tc to ctx and returns the derived context.
// This is called by the Tenant Resolution middleware after successful
// tenant identification.
func WithContext(ctx context.Context, tc TenantContext) context.Context {
	return context.WithValue(ctx, contextKey{}, tc)
}

// FromContext extracts the TenantContext from ctx.
//
// FromContext panics when no TenantContext is present. This is intentional:
// absent TenantContext is a framework bug (middleware ran in the wrong order
// or was skipped), and panicking surfaces it immediately rather than
// proceeding with no tenant identity.
func FromContext(ctx context.Context) TenantContext {
	tc, ok := ctx.Value(contextKey{}).(TenantContext)
	if !ok {
		panic(
			"runtime/tenant: TenantContext not found in context — " +
				"ensure the Tenant Resolution middleware ran before this code",
		)
	}
	return tc
}

// TryFromContext extracts the TenantContext from ctx without panicking.
// Returns (zero, false) when no TenantContext is present. Use this in
// code that may run with or without a tenant context (e.g. health checks).
func TryFromContext(ctx context.Context) (TenantContext, bool) {
	tc, ok := ctx.Value(contextKey{}).(TenantContext)
	return tc, ok
}

// IDFromContext is a convenience wrapper that returns only the TenantID.
// Panics if no TenantContext is present — same semantics as FromContext.
func IDFromContext(ctx context.Context) uuid.UUID {
	return FromContext(ctx).TenantID
}

// SystemContext returns a context that carries a sentinel TenantContext with
// TenantID == uuid.Nil, indicating a platform-level (cross-tenant) operation.
// Use for background jobs that operate on platform data, not tenant data.
//
// Repository operations in a SystemContext bypass RLS via the
// set_tenant_context() stored procedure with a special platform admin token.
func SystemContext(ctx context.Context) context.Context {
	return WithContext(ctx, TenantContext{
		TenantID: uuid.Nil,
		Locale:   "en-KE",
		Timezone: "Africa/Nairobi",
	})
}

// IsSystemContext reports whether ctx carries a platform-level (non-tenant)
// context.
func IsSystemContext(ctx context.Context) bool {
	tc, ok := TryFromContext(ctx)
	if !ok {
		return false
	}
	return tc.TenantID == uuid.Nil
}

// String returns a loggable representation (never logs sensitive data).
func (tc TenantContext) String() string {
	return fmt.Sprintf("tenant{id=%s slug=%s locale=%s}", tc.TenantID, tc.TenantSlug, tc.Locale)
}

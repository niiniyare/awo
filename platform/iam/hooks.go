package iam

import (
	"context"
	"crypto/sha256"
	"fmt"

	"awo.so/awo/auth"
	"awo.so/awo/def"
	"awo.so/awo/runtime"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the bcrypt work factor. 12 is the OWASP recommended minimum
// for 2024+; each increment doubles the computation time.
const bcryptCost = 12

// ── UserPasswordHasher ────────────────────────────────────────────────────────

// UserPasswordHasher replaces the plain-text value in password_hash with a
// bcrypt digest before the record is persisted. Registered on both BeforeCreate
// and BeforeUpdate.
//
// Callers submit the plain-text password in the password_hash field.
// On Update, if password_hash is absent or empty, the field is left unchanged
// (the existing hash from the previous record is restored).
type UserPasswordHasher struct{}

var (
	_ def.BeforeCreateHook = (*UserPasswordHasher)(nil)
	_ def.BeforeUpdateHook = (*UserPasswordHasher)(nil)
)

func (h *UserPasswordHasher) BeforeCreate(ctx context.Context, record *def.EntityRecord) error {
	return h.hashPassword(record)
}

func (h *UserPasswordHasher) BeforeUpdate(ctx context.Context, record *def.EntityRecord, prev *def.EntityRecord) error {
	plain := record.GetString("password_hash")
	if plain == "" {
		// Password not being changed — restore the existing hash.
		if prev != nil {
			record.Set("password_hash", prev.GetString("password_hash"))
		}
		return nil
	}
	return h.hashPassword(record)
}

func (h *UserPasswordHasher) hashPassword(record *def.EntityRecord) error {
	plain := record.GetString("password_hash")
	if plain == "" {
		return &runtime.ValidationError{
			Fields: map[string]string{"password_hash": "Password is required."},
		}
	}
	if len(plain) < 8 {
		return &runtime.ValidationError{
			Fields: map[string]string{"password_hash": "Password must be at least 8 characters."},
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return fmt.Errorf("iam: hash password for user %s: %w", record.ID, err)
	}
	record.Set("password_hash", string(hash))
	return nil
}

// ── UserRoleChangeHook ────────────────────────────────────────────────────────

// UserRoleChangeHook runs after a user_role record is created or deleted and
// revokes all active sessions for the affected user.
//
// When a user's role set changes, their existing sessions carry stale role data.
// The Awo session model (ADR-004) specifies that roles are loaded once at login
// and baked into the session. Role changes take effect only when the user's
// sessions are revoked and they re-authenticate.
//
// # Session revocation mechanism
//
// Sessions are enumerated via [auth.SessionStore.ListUserTokens] (which reads the
// per-user index) then deleted in bulk via [auth.SessionStore.DeleteAll]. The
// hook does not interact with the session store directly — it delegates entirely
// to the SessionStore abstraction.
//
// # Transaction boundary note
//
// AfterCreate and AfterDelete hooks run INSIDE the database transaction. Session
// deletion via SessionStore is NOT transactional with the database. If the store
// succeeds but the DB transaction later rolls back (e.g. due to a subsequent
// hook), sessions are invalidated but the role change is not applied. This is
// the safe-fail outcome: the user re-logs in and continues with their previous
// roles unchanged.
type UserRoleChangeHook struct {
	Sessions auth.SessionStore
}

var (
	_ def.AfterCreateHook = (*UserRoleChangeHook)(nil)
	_ def.AfterDeleteHook = (*UserRoleChangeHook)(nil)
)

func (h *UserRoleChangeHook) AfterCreate(ctx context.Context, record *def.EntityRecord) error {
	return h.revokeUserSessions(ctx, record.TenantID, record.GetUUID("user_id"))
}

func (h *UserRoleChangeHook) AfterDelete(ctx context.Context, record *def.EntityRecord) error {
	return h.revokeUserSessions(ctx, record.TenantID, record.GetUUID("user_id"))
}

// revokeUserSessions enumerates all active sessions for the user and deletes
// them via the SessionStore.
func (h *UserRoleChangeHook) revokeUserSessions(ctx context.Context, tenantID, userID uuid.UUID) error {
	tokens, err := h.Sessions.ListUserTokens(ctx, tenantID, userID)
	if err != nil {
		return fmt.Errorf("iam: revoke sessions for user %s: list tokens: %w", userID, err)
	}
	if len(tokens) == 0 {
		return nil
	}
	if err := h.Sessions.DeleteAll(ctx, tenantID, userID, tokens); err != nil {
		return fmt.Errorf("iam: revoke sessions for user %s: delete: %w", userID, err)
	}
	return nil
}

// ── LoginAuditImmutableGuard ──────────────────────────────────────────────────

// LoginAuditImmutableGuard prevents update and delete operations on
// iam_login_audit records. Login audit is append-only by architecture.
// This hook provides defense-in-depth alongside the PermissionSet having no
// Write or Delete permission identifiers declared.
type LoginAuditImmutableGuard struct{}

var (
	_ def.BeforeUpdateHook = (*LoginAuditImmutableGuard)(nil)
	_ def.BeforeDeleteHook = (*LoginAuditImmutableGuard)(nil)
)

func (g *LoginAuditImmutableGuard) BeforeUpdate(_ context.Context, _ *def.EntityRecord, _ *def.EntityRecord) error {
	return &runtime.BusinessError{
		Code:    "iam.login_audit.immutable",
		Message: "Login audit records are immutable and cannot be modified.",
		Status:  403,
	}
}

func (g *LoginAuditImmutableGuard) BeforeDelete(_ context.Context, _ *def.EntityRecord) error {
	return &runtime.BusinessError{
		Code:    "iam.login_audit.immutable",
		Message: "Login audit records are immutable and cannot be deleted.",
		Status:  403,
	}
}

// ── tokenHash ─────────────────────────────────────────────────────────────────

// tokenHash returns the lowercase hex SHA-256 digest of token. Used to convert
// raw session tokens and API tokens to their stored (non-reversible) form before
// database lookup or comparison.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}

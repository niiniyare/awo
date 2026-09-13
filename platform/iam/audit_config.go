package iam

import "awo.so/awo/audit"

// init registers EntityAuditConfig for all IAM entities.
//
// IAM entities carry CategoryAdmin — audit write failures propagate (ADR-017):
// any failure to audit an IAM mutation causes the mutation to roll back,
// ensuring no IAM change escapes the audit trail.
//
// iam_session and iam_login_audit use CategoryAuth — their audit writes are
// best-effort (failure suppressed) because sessions are already committed to
// Redis before the audit write runs (AUTH events are standalone, not in an
// entity TX).
//
// Sensitive fields are declared per entity so the Sanitizer strips them from
// BeforeData/AfterData before any INSERT into platform_audit_log. The
// FieldDef.Sensitive flag on password_hash is the primary guard; these entries
// are defense-in-depth for the snapshot path.
func init() {
	// Human user — ADMIN category; password hash must never appear in audit.
	audit.Register(audit.EntityAuditConfig{
		EntityName:                "iam_user",
		Enabled:                   true,
		Category:                  audit.CategoryAdmin,
		AdditionalSensitiveFields: []string{"password_hash", "totp_secret"},
	})

	// User ↔ role assignment — ADMIN; role changes are high-impact.
	audit.Register(audit.EntityAuditConfig{
		EntityName: "iam_user_role",
		Enabled:    true,
		Category:   audit.CategoryAdmin,
	})

	// Service account — ADMIN; machine principals are security-sensitive.
	audit.Register(audit.EntityAuditConfig{
		EntityName: "iam_service_account",
		Enabled:    true,
		Category:   audit.CategoryAdmin,
	})

	// API token — SECURITY; token hashes must never appear in audit.
	// CategorySecurity → failure propagates; no API token change may escape trail.
	audit.Register(audit.EntityAuditConfig{
		EntityName:                "iam_api_token",
		Enabled:                   true,
		Category:                  audit.CategorySecurity,
		AdditionalSensitiveFields: []string{"token_hash", "token"},
	})

	// Session SQL record — AUTH; best-effort (session already live in Redis).
	audit.Register(audit.EntityAuditConfig{
		EntityName: "iam_session",
		Enabled:    true,
		Category:   audit.CategoryAuth,
	})

	// Login audit entity — AUTH; legacy table, audit best-effort.
	audit.Register(audit.EntityAuditConfig{
		EntityName: "iam_login_audit",
		Enabled:    true,
		Category:   audit.CategoryAuth,
	})
}

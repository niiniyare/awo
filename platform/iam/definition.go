package iam

import "awo.so/awo/def"

// UserDefinition — authenticated human principal. One record per user per tenant.
//
// Mandatory system entity: IAM data must not be stored in JSONB (data-corruption risk
// under JSONB schema drift). password_hash is Sensitive: excluded from all standard
// API responses and structured logs. It is populated exclusively by [UserPasswordHasher].
var UserDefinition = def.SystemDefinition{
	Name:        "user",
	Module:      "iam",
	Label:       "User",
	LabelPlural: "Users",
	Description: "Authenticated human principal within this tenant.",
	Fields: []def.FieldDef{
		{
			Name:       "email",
			Type:       def.FieldTypeData,
			Label:      "Email",
			Required:   true,
			Unique:     true,
			Searchable: true,
			MaxLen:     254, // RFC 5321 maximum
		},
		{
			Name:       "full_name",
			Type:       def.FieldTypeData,
			Label:      "Full Name",
			Required:   true,
			Searchable: true,
			MaxLen:     255,
		},
		{
			// password_hash stores the bcrypt digest of the user's password.
			// The raw password is never stored. BeforeCreate/BeforeUpdate hooks
			// (UserPasswordHasher) replace the incoming plain-text value with the hash.
			Name:      "password_hash",
			Type:      def.FieldTypeData,
			Label:     "Password Hash",
			Sensitive: true,
			Hidden:    true,
			MaxLen:    72, // bcrypt output is always 60 bytes; 72 provides margin
		},
		{
			Name:    "status",
			Type:    def.FieldTypeSelect,
			Label:   "Status",
			Options: []string{"active", "inactive", "locked"},
			Default: func() any { return "active" },
		},
		{
			Name:    "email_verified",
			Type:    def.FieldTypeBool,
			Label:   "Email Verified",
			Default: func() any { return false },
		},
		{
			// last_login_at is written by AuthService.Login; not writable via API.
			Name:     "last_login_at",
			Type:     def.FieldTypeDateTime,
			Label:    "Last Login At",
			ReadOnly: true,
		},
	},
	Hooks: def.HookSet{
		BeforeCreate: []def.BeforeCreateHook{&UserPasswordHasher{}},
		BeforeUpdate: []def.BeforeUpdateHook{&UserPasswordHasher{}},
	},
	Permissions: def.PermissionSet{
		Create: []string{"iam.user.create"},
		Read:   []string{"iam.user.read"},
		Write:  []string{"iam.user.update"},
		Delete: []string{"iam.user.delete"},
		Actions: map[string][]string{
			"reset_password": {"iam.user.reset_password"},
			"lock":           {"iam.user.lock"},
			"unlock":         {"iam.user.unlock"},
		},
		// Viewers see only their own record; tenant.admin sees all users.
		Policy: userSelfOrAdminPolicy,
	},
}

// userRoleChangeHook is the singleton hook instance wired at startup.
// Set by [New] before the first request is handled.
var userRoleChangeHook = &UserRoleChangeHook{}

// UserRoleDefinition — tenant-scoped assignment of a platform role to a user.
//
// One record per (user_id, role_name) pair within a tenant. AfterCreate and
// AfterDelete hooks revoke the affected user's active Redis sessions, forcing
// re-login so the updated role set is reflected in the new session.
//
// Write permission is intentionally absent: role assignments may not be patched,
// only created or deleted (immutable join records).
var UserRoleDefinition = def.SystemDefinition{
	Name:        "user_role",
	Module:      "iam",
	Label:       "User Role",
	LabelPlural: "User Roles",
	Description: "Assignment of a platform role to a user within this tenant.",
	Fields: []def.FieldDef{
		{
			Name:       "user_id",
			Type:       def.FieldTypeLink,
			Label:      "User",
			LinkTarget: "iam_user",
			Required:   true,
			Immutable:  true,
		},
		{
			// role_name stores the canonical role identifier, e.g. "role:tenant.admin".
			// References iam_roles.name (global table, not an EntityDefinition).
			Name:      "role_name",
			Type:      def.FieldTypeData,
			Label:     "Role",
			Required:  true,
			Immutable: true,
			MaxLen:    128,
		},
		{
			Name:       "granted_by_id",
			Type:       def.FieldTypeLink,
			Label:      "Granted By",
			LinkTarget: "iam_user",
			ReadOnly:   true,
		},
	},
	Hooks: def.HookSet{
		AfterCreate: []def.AfterCreateHook{userRoleChangeHook},
		AfterDelete: []def.AfterDeleteHook{userRoleChangeHook},
	},
	Permissions: def.PermissionSet{
		Create: []string{"iam.user_role.create"},
		Read:   []string{"iam.user_role.read"},
		// No Write — role assignments are immutable. Remove and re-create to change.
		Delete: []string{"iam.user_role.delete"},
	},
}

// ServiceAccountDefinition — machine principal for API-to-API integrations.
//
// Service accounts authenticate via long-lived API tokens ([APITokenDefinition]).
// Their sessions are stateless (no Redis entry); each request validates against
// a 60-second Redis cache of the token hash lookup.
var ServiceAccountDefinition = def.SystemDefinition{
	Name:        "service_account",
	Module:      "iam",
	Label:       "Service Account",
	LabelPlural: "Service Accounts",
	Description: "Machine principal for API-to-API integrations. Authenticates via API tokens.",
	Fields: []def.FieldDef{
		{
			Name:      "name",
			Type:      def.FieldTypeData,
			Label:     "Name",
			Required:  true,
			Immutable: true,
			Unique:    true,
			MaxLen:    128,
		},
		{
			Name:  "description",
			Type:  def.FieldTypeSmallText,
			Label: "Description",
		},
		{
			Name:    "status",
			Type:    def.FieldTypeSelect,
			Label:   "Status",
			Options: []string{"active", "inactive"},
			Default: func() any { return "active" },
		},
	},
	Permissions: def.PermissionSet{
		Create: []string{"iam.service_account.create"},
		Read:   []string{"iam.service_account.read"},
		Write:  []string{"iam.service_account.update"},
		Delete: []string{"iam.service_account.delete"},
	},
}

// APITokenDefinition — long-lived API key tied to a service account.
//
// The raw token is generated by [AuthService.CreateAPIToken], returned to the
// caller exactly once, and never stored. Only the SHA-256 hex digest (token_hash)
// is persisted. Sensitive: excluded from all API responses and logs.
//
// Write permission is absent — tokens are immutable after creation. Revocation
// is via the "revoke" custom action or deletion.
var APITokenDefinition = def.SystemDefinition{
	Name:        "api_token",
	Module:      "iam",
	Label:       "API Token",
	LabelPlural: "API Tokens",
	Description: "Long-lived API key for a service account. Raw token returned only at creation.",
	Fields: []def.FieldDef{
		{
			Name:       "service_account_id",
			Type:       def.FieldTypeLink,
			Label:      "Service Account",
			LinkTarget: "iam_service_account",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:     "name",
			Type:     def.FieldTypeData,
			Label:    "Token Name",
			Required: true,
			MaxLen:   128,
		},
		{
			// token_hash is the SHA-256 hex digest of the raw API token.
			// The raw token is never stored. Hash comparison happens at auth time.
			Name:      "token_hash",
			Type:      def.FieldTypeData,
			Label:     "Token Hash",
			Sensitive: true,
			Immutable: true,
			Hidden:    true,
			MaxLen:    64, // SHA-256 hex: always 64 characters
		},
		{
			Name:  "expires_at",
			Type:  def.FieldTypeDateTime,
			Label: "Expires At",
		},
		{
			Name:     "last_used_at",
			Type:     def.FieldTypeDateTime,
			Label:    "Last Used At",
			ReadOnly: true,
		},
		{
			Name:    "is_revoked",
			Type:    def.FieldTypeBool,
			Label:   "Revoked",
			Default: func() any { return false },
		},
	},
	Permissions: def.PermissionSet{
		Create: []string{"iam.api_token.create"},
		Read:   []string{"iam.api_token.read"},
		// No Write — tokens are immutable.
		Delete: []string{"iam.api_token.delete"},
		Actions: map[string][]string{
			"revoke": {"iam.api_token.revoke"},
		},
	},
}

// SessionDefinition — SQL audit trail for authentication sessions.
//
// This is the PostgreSQL session record, distinct from the Redis session value.
// Redis holds the live session for sub-millisecond validation on every request;
// this table holds the immutable audit trail for forced-logout, anomaly
// detection, compliance reporting, and forensic investigation.
//
// token_hash is the SHA-256 hex digest of the session token. The raw token is
// never stored in SQL. Sensitive: excluded from all API responses and logs.
//
// Write permission is absent — sessions are immutable after creation except for
// revoked_at, which is set via the "revoke" action only.
var SessionDefinition = def.SystemDefinition{
	Name:        "session",
	Module:      "iam",
	Label:       "Session",
	LabelPlural: "Sessions",
	Description: "Audit record for an authentication session. Immutable after creation except revoked_at.",
	Fields: []def.FieldDef{
		{
			Name:      "token_hash",
			Type:      def.FieldTypeData,
			Label:     "Token Hash",
			Sensitive: true,
			Immutable: true,
			Hidden:    true,
			MaxLen:    64,
		},
		{
			Name:       "user_id",
			Type:       def.FieldTypeLink,
			Label:      "User",
			LinkTarget: "iam_user",
			Immutable:  true,
		},
		{
			Name:       "service_account_id",
			Type:       def.FieldTypeLink,
			Label:      "Service Account",
			LinkTarget: "iam_service_account",
			Immutable:  true,
		},
		{
			Name:      "issued_at",
			Type:      def.FieldTypeDateTime,
			Label:     "Issued At",
			Required:  true,
			Immutable: true,
		},
		{
			Name:      "expires_at",
			Type:      def.FieldTypeDateTime,
			Label:     "Expires At",
			Required:  true,
			Immutable: true,
		},
		{
			// revoked_at is nil for active/expired sessions; set on explicit revocation.
			Name:  "revoked_at",
			Type:  def.FieldTypeDateTime,
			Label: "Revoked At",
		},
		{
			Name:      "device_id",
			Type:      def.FieldTypeData,
			Label:     "Device ID",
			Immutable: true,
			MaxLen:    255,
		},
		{
			Name:      "ip_address",
			Type:      def.FieldTypeData,
			Label:     "IP Address",
			Immutable: true,
			MaxLen:    45, // IPv6 max length
		},
		{
			// metadata stores extensible per-session data (device info, OAuth claims,
			// MFA method used, etc.). Framework reserves keys prefixed "awo:".
			// Never store secrets here — this column is readable by tenant.admin.
			Name:        "metadata",
			Type:        def.FieldTypeJSON,
			Label:       "Session Metadata",
			Description: "Extensible per-session JSONB payload. Framework keys use the 'awo:' prefix.",
			Hidden:      true, // excluded from list views; present in inspect/detail
		},
	},
	Permissions: def.PermissionSet{
		Read:   []string{"iam.session.read"},
		Delete: []string{"iam.session.delete"},
		Actions: map[string][]string{
			"revoke": {"iam.session.revoke"},
		},
		// Users see only their own sessions; tenant.admin sees all sessions.
		Policy: sessionOwnerPolicy,
	},
}

// LoginAuditDefinition — tamper-evident log of authentication events.
//
// Append-only: Create is permitted (by the auth service only, not via API),
// Update and Delete are prohibited. This is enforced at two levels:
//  1. PermissionSet: no Write or Delete identifiers declared.
//  2. Hooks: [LoginAuditImmutableGuard] returns a BusinessError on any attempt.
//
// The auth service writes records directly via the repository, not via the
// EntityDefinition API route, so no Create permission identifier is declared here.
var LoginAuditDefinition = def.SystemDefinition{
	Name:        "login_audit",
	Module:      "iam",
	Label:       "Login Audit",
	LabelPlural: "Login Audits",
	Description: "Tamper-evident append-only log of authentication events. Cannot be modified or deleted.",
	Fields: []def.FieldDef{
		{
			Name:     "event",
			Type:     def.FieldTypeSelect,
			Label:    "Event",
			Required: true,
			Options:  []string{"login", "logout", "failed_login", "token_use", "session_revoked"},
		},
		{
			Name:       "user_id",
			Type:       def.FieldTypeLink,
			Label:      "User",
			LinkTarget: "iam_user",
		},
		{
			Name:       "service_account_id",
			Type:       def.FieldTypeLink,
			Label:      "Service Account",
			LinkTarget: "iam_service_account",
		},
		{
			Name:   "ip_address",
			Type:   def.FieldTypeData,
			Label:  "IP Address",
			MaxLen: 45,
		},
		{
			Name:   "device_id",
			Type:   def.FieldTypeData,
			Label:  "Device ID",
			MaxLen: 255,
		},
		{
			// failure_reason is set for failed_login events; empty for successful events.
			Name:   "failure_reason",
			Type:   def.FieldTypeData,
			Label:  "Failure Reason",
			MaxLen: 255,
		},
	},
	Hooks: def.HookSet{
		BeforeUpdate: []def.BeforeUpdateHook{&LoginAuditImmutableGuard{}},
		BeforeDelete: []def.BeforeDeleteHook{&LoginAuditImmutableGuard{}},
	},
	Permissions: def.PermissionSet{
		// No Create — records inserted directly by AuthService, not via API.
		// No Write/Delete — append-only. Guards enforce this at the hook level too.
		Read: []string{"iam.login_audit.read"},
		// Only tenant.admin can read audit logs.
		Policy: loginAuditTenantAdminPolicy,
	},
}

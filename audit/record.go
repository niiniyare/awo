package audit

import (
	"time"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

// OperationType classifies the mutation that produced the audit record.
type OperationType string

const (
	OperationCreate OperationType = "create"
	OperationUpdate OperationType = "update"
	OperationDelete OperationType = "delete"
	OperationAction OperationType = "action"
	OperationLogin  OperationType = "login"
	OperationLogout OperationType = "logout"
	OperationSystem OperationType = "system"
)

// EventCategory classifies the audit event for failure policy routing,
// retention policy, and compliance reporting.
type EventCategory string

const (
	// CategoryData covers standard entity CRUD mutations.
	CategoryData EventCategory = "DATA"
	// CategoryAuth covers login, logout, password reset, session events.
	CategoryAuth EventCategory = "AUTH"
	// CategoryAccess covers permission-denied events.
	CategoryAccess EventCategory = "ACCESS"
	// CategoryAdmin covers privileged administrative operations.
	CategoryAdmin EventCategory = "ADMIN"
	// CategoryWorkflow covers Temporal workflow lifecycle events.
	CategoryWorkflow EventCategory = "WORKFLOW"
	// CategorySystem covers background system operations (outbox relay, cron).
	CategorySystem EventCategory = "SYSTEM"
	// CategoryOutbound covers external API calls and event publication.
	CategoryOutbound EventCategory = "OUTBOUND"
	// CategorySecurity covers security-sensitive events (token revocation, etc).
	CategorySecurity EventCategory = "SECURITY"
)

// Severity classifies the risk level of the audit event.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

// SystemActor identifies non-human execution contexts for audit records
// produced outside of a user or service-account request.
//
// Exactly one of AuditRecord.Actor or AuditRecord.SystemActor is non-nil/
// non-empty per record.
type SystemActor string

const (
	SystemBootstrap   SystemActor = "system:bootstrap"
	SystemMigration   SystemActor = "system:migration"
	SystemOutboxRelay SystemActor = "system:outbox-relay"
	SystemScheduler   SystemActor = "system:scheduler"
)

// AuditRecord is the unit written to platform_audit_log for every auditable
// event. It is constructed by the runtime pipeline or by IAM/auth services
// and passed to an AuditWriter.
//
// Invariant: exactly one of Actor or SystemActor is set per record.
// Actor is set for human user and service-account requests.
// SystemActor is set for background/system operations.
type AuditRecord struct {
	// Identity
	ID        uuid.UUID
	TenantID  uuid.UUID
	RequestID string // X-Request-ID from middleware; empty for system ops

	// Entity
	EntityName string    // qualified name e.g. "finance_invoice"
	RecordID   uuid.UUID // primary key of the affected record; zero for session/access events
	Operation  OperationType

	// Actor — exactly one of Actor or SystemActor is set
	Actor       *def.Actor  // nil for system operations
	SystemActor SystemActor // empty for human/service-account operations

	// Request context (populated for HTTP-originated events)
	IPAddress string
	SessionID string // HMAC-SHA256(session_token, server_secret); never the raw token

	// Snapshots — sensitive fields stripped before population
	BeforeData    map[string]any // nil on Create; nil for AUTH/ACCESS events
	AfterData     map[string]any // nil on Delete; nil for AUTH/ACCESS events
	ChangedFields []string       // populated on Update only; computed from stripped maps

	// Classification
	EventCategory   EventCategory
	Severity        Severity
	RiskScore       int             // 0–100 composite score
	ComplianceFlags map[string]bool // "GDPR": true, "PCI_DSS": false, "KRA_ETIMS": true

	// Event-type-specific context (JSONB; schema by EventCategory)
	Context map[string]any

	CreatedAt time.Time
}

// ActorID returns the actor's user ID for storage. Returns uuid.Nil for system
// operations or service-account-only operations.
func (r *AuditRecord) ActorID() uuid.UUID {
	if r.Actor != nil {
		return r.Actor.UserID
	}
	return uuid.Nil
}

// ServiceAccountID returns the service account ID for storage. Returns
// uuid.Nil when the actor is a human user or a system actor.
func (r *AuditRecord) ServiceAccountID() uuid.UUID {
	if r.Actor != nil {
		return r.Actor.ServiceAccountID
	}
	return uuid.Nil
}

// Validate checks the record for mandatory field invariants.
// Returns a non-nil error describing the first violation found.
func (r *AuditRecord) Validate() error {
	if r.TenantID == uuid.Nil {
		return errorf("AuditRecord.TenantID must not be nil")
	}
	if r.EntityName == "" {
		return errorf("AuditRecord.EntityName must not be empty")
	}
	if r.Operation == "" {
		return errorf("AuditRecord.Operation must not be empty")
	}
	if r.EventCategory == "" {
		return errorf("AuditRecord.EventCategory must not be empty")
	}
	// Exactly one of Actor or SystemActor must be set.
	hasActor := r.Actor != nil
	hasSystem := r.SystemActor != ""
	if hasActor == hasSystem { // both set or both absent
		if hasActor {
			return errorf("AuditRecord: Actor and SystemActor are mutually exclusive; only one may be set")
		}
		return errorf("AuditRecord: one of Actor or SystemActor must be set")
	}
	return nil
}

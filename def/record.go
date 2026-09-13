package def

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// EntityRecord is the in-flight representation of a single entity record
// during the hook pipeline. It carries the raw field values as a generic
// map so that hooks written against the def package do not need to know
// the concrete Go struct for each entity.
//
// The runtime populates EntityRecord before hook execution and flushes it to
// the store layer after the after_save stage completes.
type EntityRecord struct {
	// ID is the UUIDv7 primary key. Set by the runtime on Create; pre-set to
	// the existing record ID on Update and Delete.
	ID uuid.UUID

	// TenantID is the UUID of the owning tenant. Always set by the middleware
	// pipeline before any hook executes.
	TenantID uuid.UUID

	// EntityName is the stable name declared on the EntityDefinition.
	EntityName string

	// Data holds all field values keyed by field name. Types match the
	// FieldType semantics:
	//   FieldTypeData        → string
	//   FieldTypeInt         → int64
	//   FieldTypeFloat       → float64
	//   FieldTypeCurrency    → decimal.Decimal
	//   FieldTypeBool        → bool
	//   FieldTypeDate        → time.Time (date part only)
	//   FieldTypeDateTime    → time.Time
	//   FieldTypeSelect      → string
	//   FieldTypeMultiSelect → []string
	//   FieldTypeLink        → uuid.UUID
	//   FieldTypeLinkList    → []uuid.UUID
	//   FieldTypeJSON        → map[string]any
	//   FieldTypeNamingSeries→ string (populated by AfterCreate hook)
	Data map[string]any

	// CustomFields holds values for fields added at runtime via the Metadata
	// module. Stored in the custom_fields JSONB column.
	CustomFields map[string]any

	// Meta carries non-persisted context for hooks: the Actor who initiated
	// the operation, workflow input hints, etc.
	Meta RecordMeta

	// CreatedAt and UpdatedAt are set by the runtime, not by module code.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Get retrieves a field value from Data, returning nil if the key is absent.
func (r *EntityRecord) Get(field string) any {
	if r.Data == nil {
		return nil
	}
	return r.Data[field]
}

// Set stores a field value in Data, initialising the map if necessary.
func (r *EntityRecord) Set(field string, value any) {
	if r.Data == nil {
		r.Data = make(map[string]any)
	}
	r.Data[field] = value
}

// GetString returns the field value as a string, or "" if absent or wrong type.
func (r *EntityRecord) GetString(field string) string {
	v, _ := r.Data[field].(string)
	return v
}

// GetInt returns the field value as int64, or 0 if absent or wrong type.
func (r *EntityRecord) GetInt(field string) int64 {
	v, _ := r.Data[field].(int64)
	return v
}

// GetDecimal returns the field value as decimal.Decimal, or zero if absent.
func (r *EntityRecord) GetDecimal(field string) decimal.Decimal {
	v, _ := r.Data[field].(decimal.Decimal)
	return v
}

// GetUUID returns the field value as uuid.UUID, or uuid.Nil if absent.
func (r *EntityRecord) GetUUID(field string) uuid.UUID {
	v, _ := r.Data[field].(uuid.UUID)
	return v
}

// RecordMeta carries non-persisted context attached to an EntityRecord during
// the hook pipeline.
type RecordMeta struct {
	// Actor is the authenticated user or service account that initiated the
	// operation. Nil for background operations using a SystemViewer context.
	Actor *Actor

	// OperationType identifies whether this is a Create, Update, or Delete.
	OperationType OperationType

	// RequestID is the X-Request-ID value from the incoming HTTP request.
	RequestID string
}

// Actor represents the authenticated principal performing an operation.
// It is constructed from [auth.Session] at request time and embedded in the
// request context. Hooks and action handlers receive it via RecordMeta.Actor
// or ActionContext.
//
// ADR-003: IsPlatformAdmin is a method, not a field. ServiceAccountID
// replaces the removed IsPlatformAdmin bool. Privileges belong in Roles.
type Actor struct {
	// UserID is the UUID of the authenticated human user.
	// uuid.Nil for service account sessions.
	UserID uuid.UUID

	// ServiceAccountID is the UUID of the authenticated service account.
	// uuid.Nil for human user sessions. Exactly one of UserID and
	// ServiceAccountID is non-nil for any given Actor.
	ServiceAccountID uuid.UUID

	// TenantID is the tenant the actor is operating within.
	TenantID uuid.UUID

	// Roles is the complete set of role names held by this principal within
	// the current tenant scope. Format: "role:{domain}.{name}".
	Roles []string
}

// IsPlatformAdmin reports whether this actor holds the platform-admin role,
// which bypasses all PolicyEvaluator checks unconditionally (ADR-003).
func (a *Actor) IsPlatformAdmin() bool {
	for _, r := range a.Roles {
		if r == "role:platform-admin" {
			return true
		}
	}
	return false
}

// IsServiceAccount reports whether this actor is a machine/service principal.
func (a *Actor) IsServiceAccount() bool {
	return a.ServiceAccountID != uuid.Nil
}

// HasRole reports whether this actor holds the named role (exact match).
func (a *Actor) HasRole(role string) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// OperationType identifies the mutation being performed.
type OperationType string

const (
	OperationCreate OperationType = "create"
	OperationUpdate OperationType = "update"
	OperationDelete OperationType = "delete"
)

// TriggerContext is passed to [WorkflowTrigger.InputBuilder]. It gives the
// builder access to the actor and runtime metadata without importing runtime
// packages.
type TriggerContext struct {
	Actor     *Actor
	RequestID string
	TenantID  uuid.UUID
}

// ActionContext is the parameter received by [ActionDef.HandlerFunc].
// It provides a permission-scoped runtime, the target record ID, and the
// actor — all pre-resolved by the framework before the handler is called.
type ActionContext struct {
	// Ctx is the request context, carrying TenantContext and cancellation.
	Ctx context.Context

	// RecordID is the UUID of the entity record the action targets.
	RecordID uuid.UUID

	// Actor is the authenticated principal. Never nil for protected actions.
	Actor *Actor

	// Body is the raw JSON body of the action request, if any.
	Body []byte

	// Runtime provides access to all framework services: repositories,
	// transactions, event publishing, workflow start, notifications, cache,
	// clock, and logger. Constructed and injected by the framework before
	// the handler is called. Never nil.
	Runtime ActionRuntime
}

// ActionResult is returned by [ActionDef.HandlerFunc] to communicate the
// outcome to the caller.
type ActionResult struct {
	// Message is a human-readable summary of the result (e.g. "Invoice
	// submitted").
	Message string

	// Data is an optional payload returned to the API client. Must be JSON-
	// serialisable.
	Data any

	// WorkflowID is set when the action triggered a Temporal workflow, allowing
	// the client to poll or subscribe.
	WorkflowID string
}

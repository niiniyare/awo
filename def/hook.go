package def

import "context"

// HookSet groups all lifecycle hooks for an entity. Each stage is a slice,
// allowing multiple hooks to be composed. Hooks within a stage execute in
// declaration order.
//
// Hook pipeline stages and their transaction boundary:
//
//	ASSEMBLE → before_validate → VALIDATE → AUTHORIZE
//	         → before_save (TX NOT YET open)
//	         → [TX begins]
//	         → PERSIST
//	         → after_save (inside TX — error causes rollback)
//	         → [TX commits]
//	         → Workflow start (outside TX)
//
// Hooks must not assume a database transaction is open unless the stage
// documentation explicitly states "inside TX".
type HookSet struct {
	// BeforeValidate hooks run before field-level validation. Use to normalise
	// or derive field values before validation rules are checked.
	// NOT inside TX. Abort via returning a ValidationError.
	BeforeValidate []BeforeValidateHook

	// BeforeCreate hooks run after validation on Create operations only.
	// NOT inside TX. Abort via returning a BusinessError.
	BeforeCreate []BeforeCreateHook

	// AfterCreate hooks run after the record is persisted on Create operations.
	// INSIDE TX. Error causes rollback. Use for within-transaction side effects
	// (e.g. writing a journal entry, triggering an outbox event).
	AfterCreate []AfterCreateHook

	// BeforeUpdate hooks run after validation on Update operations only.
	// NOT inside TX. Abort via returning a BusinessError.
	BeforeUpdate []BeforeUpdateHook

	// AfterUpdate hooks run after the record is updated.
	// INSIDE TX. Error causes rollback.
	AfterUpdate []AfterUpdateHook

	// BeforeDelete hooks run before the record is deleted.
	// NOT inside TX. Abort via returning a BusinessError.
	BeforeDelete []BeforeDeleteHook

	// AfterDelete hooks run after the record is deleted.
	// INSIDE TX. Error causes rollback.
	AfterDelete []AfterDeleteHook

	// BeforeSave runs on both Create and Update, after type-specific before_*
	// hooks. NOT inside TX.
	BeforeSave []BeforeSaveHook

	// AfterSave runs on both Create and Update, after PERSIST.
	// INSIDE TX. Error causes rollback.
	AfterSave []AfterSaveHook
}

// BeforeValidateHook normalises or derives field values before validation.
//
// Implementations must not perform database writes. Read operations are
// allowed but incur a latency cost on every mutation.
type BeforeValidateHook interface {
	BeforeValidate(ctx context.Context, record *EntityRecord) error
}

// BeforeCreateHook enforces pre-create business rules.
//
// Returning a non-nil error aborts the Create operation. Wrap domain
// violations in *BusinessError for structured client responses.
type BeforeCreateHook interface {
	BeforeCreate(ctx context.Context, record *EntityRecord) error
}

// AfterCreateHook performs within-transaction side effects after a record is
// persisted for the first time.
//
// INSIDE TX: returning a non-nil error rolls back the transaction.
type AfterCreateHook interface {
	AfterCreate(ctx context.Context, record *EntityRecord) error
}

// BeforeUpdateHook enforces pre-update business rules.
//
// Returning a non-nil error aborts the Update. The hook receives the full
// record with the proposed new values already applied.
type BeforeUpdateHook interface {
	BeforeUpdate(ctx context.Context, record *EntityRecord, prev *EntityRecord) error
}

// AfterUpdateHook performs within-transaction side effects after a record is
// updated.
//
// INSIDE TX: returning a non-nil error rolls back the transaction.
type AfterUpdateHook interface {
	AfterUpdate(ctx context.Context, record *EntityRecord, prev *EntityRecord) error
}

// BeforeDeleteHook enforces pre-delete business rules (e.g. reference checks).
//
// Returning a non-nil error aborts the Delete.
type BeforeDeleteHook interface {
	BeforeDelete(ctx context.Context, record *EntityRecord) error
}

// AfterDeleteHook performs within-transaction side effects after a record is
// deleted.
//
// INSIDE TX: returning a non-nil error rolls back the transaction.
type AfterDeleteHook interface {
	AfterDelete(ctx context.Context, record *EntityRecord) error
}

// BeforeSaveHook runs on Create and Update operations, after type-specific
// before_create / before_update hooks. Use for cross-cutting concerns that
// apply to both operations.
//
// NOT inside TX.
type BeforeSaveHook interface {
	BeforeSave(ctx context.Context, record *EntityRecord) error
}

// AfterSaveHook runs on Create and Update operations, after PERSIST.
// INSIDE TX: returning a non-nil error rolls back the transaction.
type AfterSaveHook interface {
	AfterSave(ctx context.Context, record *EntityRecord) error
}

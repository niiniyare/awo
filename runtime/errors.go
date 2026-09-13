package runtime

import (
	"errors"
	"fmt"
)

// ValidationError is a field-level validation failure. It maps to HTTP 422.
// Use errors.As to unwrap from hook chains.
type ValidationError struct {
	// Fields maps field names to human-readable error messages.
	// The key "." (dot) is the convention for entity-level errors not
	// attributable to a specific field.
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %d field(s) invalid", len(e.Fields))
}

// NewValidationError creates a ValidationError with a single field error.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Fields: map[string]string{field: message}}
}

// AddField adds a field error to the ValidationError.
func (e *ValidationError) AddField(field, message string) {
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}
	e.Fields[field] = message
}

// IsEmpty reports whether the ValidationError has no fields set.
func (e *ValidationError) IsEmpty() bool {
	return len(e.Fields) == 0
}

// BusinessError is a domain rule violation. It maps to a specific HTTP status
// and carries a machine-readable error code.
//
// Use errors.As (not type switch) to unwrap from error chains:
//
//	var be *runtime.BusinessError
//	if errors.As(err, &be) {
//	    // handle
//	}
type BusinessError struct {
	// Code is the machine-readable error code. Convention:
	// "{entity}.{violation}" e.g. "invoice.already_submitted".
	Code string

	// Message is the human-readable, client-safe message.
	Message string

	// Status is the HTTP status code (400, 402, 403, 404, 409, 410, 422, 503).
	Status int
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NotFoundError signals that a requested record does not exist.
// Maps to HTTP 404.
type NotFoundError struct {
	EntityName string
	ID         string
}

func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s not found: %s", e.EntityName, e.ID)
	}
	return fmt.Sprintf("%s not found", e.EntityName)
}

// PermissionError signals that the authenticated actor lacks the required
// permission. Maps to HTTP 403.
type PermissionError struct {
	Action     string
	EntityName string
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf("permission denied: %s on %s", e.Action, e.EntityName)
}

// ImmutableFieldError signals an attempt to modify a field declared as
// Immutable: true. Maps to HTTP 422.
type ImmutableFieldError struct {
	EntityName string
	Field      string
}

func (e *ImmutableFieldError) Error() string {
	return fmt.Sprintf("%s.%s is immutable and cannot be changed after creation",
		e.EntityName, e.Field)
}

// IsNotFound reports whether err (or any error in its chain) is a NotFoundError.
func IsNotFound(err error) bool {
	var nfe *NotFoundError
	return errors.As(err, &nfe)
}

// IsValidation reports whether err (or any error in its chain) is a
// ValidationError.
func IsValidation(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// HookPanicError is returned when a lifecycle hook panics. The pipeline
// recovers the panic, wraps it in HookPanicError, and propagates it as a
// regular error so the driver can roll back any open transaction cleanly.
//
// Use errors.As to detect this error type:
//
//	var hpe *runtime.HookPanicError
//	if errors.As(err, &hpe) {
//	    // log hpe.Stage, hpe.HookName, hpe.Panic
//	}
type HookPanicError struct {
	// Stage is the hook stage that panicked (e.g. "before_validate",
	// "before_create", "after_save").
	Stage string

	// HookName is the type name of the hook implementation that panicked,
	// obtained via fmt.Sprintf("%T", hookImpl).
	HookName string

	// Panic is the value recovered from the panic.
	Panic any
}

func (e *HookPanicError) Error() string {
	return fmt.Sprintf("hook panic at stage %q (hook %s): %v", e.Stage, e.HookName, e.Panic)
}

// IsHookPanic reports whether err (or any error in its chain) is a
// HookPanicError.
func IsHookPanic(err error) bool {
	var hpe *HookPanicError
	return errors.As(err, &hpe)
}

// IsPermission reports whether err is a PermissionError.
func IsPermission(err error) bool {
	var pe *PermissionError
	return errors.As(err, &pe)
}

// IsBusiness reports whether err is a BusinessError.
func IsBusiness(err error) bool {
	var be *BusinessError
	return errors.As(err, &be)
}

// HTTPStatus returns the appropriate HTTP status code for err.
// Falls back to 500 for unrecognised errors.
func HTTPStatus(err error) int {
	if err == nil {
		return 200
	}
	var nfe *NotFoundError
	if errors.As(err, &nfe) {
		return 404
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		return 422
	}
	var pe *PermissionError
	if errors.As(err, &pe) {
		return 403
	}
	var be *BusinessError
	if errors.As(err, &be) {
		return be.Status
	}
	return 500
}

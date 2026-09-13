// Package dberr translates PostgreSQL error codes to framework domain errors.
// It is used exclusively by driver implementations — module authors never call
// this package directly.
package dberr

import (
	"errors"
	"fmt"

	"github.com/jackc/pgconn"

	"awo.so/awo/runtime"
)

// PostgreSQL error code constants relevant to framework operations.
const (
	CodeUniqueViolation      = "23505"
	CodeForeignKeyViolation  = "23503"
	CodeCheckViolation       = "23514"
	CodeNotNullViolation     = "23502"
	CodeDeadlockDetected     = "40P01"
	CodeSerializationFailure = "40001"

	// Application-defined error codes (set by stored procedures).
	CodeTenantNotFound  = "P0001"
	CodeTenantNotActive = "P0002"
)

// Parse translates a raw database error into a framework domain error.
// op is the operation name for error context (e.g. "invoice.Create").
//
// Returns the original error wrapped with op context if no specific translation
// applies, so callers always have context.
func Parse(err error, op string) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return fmt.Errorf("%s: %w", op, err)
	}

	switch pgErr.Code {
	case CodeUniqueViolation:
		// Extract the constraint name for a more specific message.
		constraint := pgErr.ConstraintName
		if constraint == "" {
			constraint = "record"
		}
		return &runtime.BusinessError{
			Code:    "duplicate",
			Message: fmt.Sprintf("A record with this %s already exists", constraint),
			Status:  409,
		}

	case CodeForeignKeyViolation:
		return &runtime.BusinessError{
			Code:    "reference_violation",
			Message: "Cannot complete operation: a referenced record does not exist or is protected",
			Status:  409,
		}

	case CodeCheckViolation:
		constraint := pgErr.ConstraintName
		return &runtime.BusinessError{
			Code:    "constraint_violation",
			Message: fmt.Sprintf("Value violates constraint: %s", constraint),
			Status:  400,
		}

	case CodeNotNullViolation:
		column := pgErr.ColumnName
		return &runtime.ValidationError{
			Fields: map[string]string{column: "this field is required"},
		}

	case CodeDeadlockDetected, CodeSerializationFailure:
		// Transient errors — caller should retry.
		return &runtime.BusinessError{
			Code:    "conflict_retry",
			Message: "Operation conflicted with a concurrent request — please retry",
			Status:  409,
		}

	case CodeTenantNotFound:
		return &runtime.BusinessError{
			Code:    "tenant.not_found",
			Message: "Tenant not found",
			Status:  404,
		}

	case CodeTenantNotActive:
		return &runtime.BusinessError{
			Code:    "tenant.not_active",
			Message: "Tenant is not active",
			Status:  503,
		}
	}

	// No specific translation — wrap with op context.
	return fmt.Errorf("%s: database error (code=%s): %w", op, pgErr.Code, err)
}

// IsTransient returns true if the error is a transient database error that the
// caller may safely retry.
func IsTransient(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == CodeDeadlockDetected || pgErr.Code == CodeSerializationFailure
}

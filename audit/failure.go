package audit

import (
	"context"
	"log/slog"
)

// FailurePolicy determines whether an AuditWriter error propagates to the
// caller (aborting the originating mutation) or is suppressed (logged and
// metered, mutation proceeds).
//
// ADR-017: ADMIN and SECURITY events propagate. All other categories suppress.
type FailurePolicy int

const (
	// FailurePolicyPropagate returns the Write error to the caller.
	// The caller must treat the error as fatal (roll back the transaction).
	FailurePolicyPropagate FailurePolicy = iota

	// FailurePolicySilentSuppress logs the error at WARN level and returns nil
	// to the caller, allowing the originating mutation to succeed.
	FailurePolicySilentSuppress
)

// PolicyFor returns the FailurePolicy for the given EventCategory.
//
// ADMIN and SECURITY → FailurePolicyPropagate.
// All other categories → FailurePolicySilentSuppress.
func PolicyFor(category EventCategory) FailurePolicy {
	switch category {
	case CategoryAdmin, CategorySecurity:
		return FailurePolicyPropagate
	default:
		return FailurePolicySilentSuppress
	}
}

// Apply calls w.Write(ctx, record). If Write returns an error, the failure
// policy for record.EventCategory determines whether the error is returned or
// suppressed. When suppressed, the error is logged at WARN level.
//
// Apply is the single call site that all framework integration points use to
// execute an audit write with correct failure semantics.
func Apply(ctx context.Context, w AuditWriter, record AuditRecord) error {
	err := w.Write(ctx, record)
	if err == nil {
		return nil
	}

	policy := PolicyFor(record.EventCategory)
	if policy == FailurePolicyPropagate {
		return err
	}

	// Suppress — log for observability, do not abort the mutation.
	slog.WarnContext(ctx, "audit write suppressed",
		"entity", record.EntityName,
		"operation", string(record.Operation),
		"category", string(record.EventCategory),
		"error", err.Error(),
	)
	return nil
}

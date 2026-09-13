package audit

import (
	"context"
	"log/slog"
	"sync/atomic"
	"unsafe"
)

// SensitiveEntityLoader returns the set of entity names that carry an
// additional risk premium during scoring. Called once by Warm(). The
// production implementation queries audit_sensitive_entities (Phase 2
// migration). Return nil, nil from a nil or no-op loader to use defaults.
//
// If the underlying table does not yet exist, the loader should return
// nil, nil — the scorer falls back to default rules (same as pre-Warm).
type SensitiveEntityLoader func(ctx context.Context) ([]string, error)

// RiskScorer computes a 0–100 risk score for an AuditRecord based on the
// operation type, actor, entity category, and runtime configuration loaded
// from the database.
//
// The scorer is safe for concurrent use. The internal state is updated
// atomically when Warm completes so in-flight Score calls always see a
// consistent snapshot.
type RiskScorer struct {
	// state is an atomically-swapped pointer to scorerState.
	// Uses unsafe.Pointer to allow lock-free reads.
	state  unsafe.Pointer // *scorerState
	loader SensitiveEntityLoader
}

// scorerState holds the immutable snapshot used during scoring.
type scorerState struct {
	// sensitiveEntities is the set of entity names that carry a risk premium.
	// Populated from audit_sensitive_fields at Warm time.
	sensitiveEntities map[string]struct{}
}

var defaultScorerState = &scorerState{
	sensitiveEntities: make(map[string]struct{}),
}

// NewRiskScorer returns a RiskScorer using default scoring rules.
// Call Warm(ctx) once after the database is reachable to load runtime config.
func NewRiskScorer() *RiskScorer {
	rs := &RiskScorer{}
	atomic.StorePointer(&rs.state, unsafe.Pointer(defaultScorerState))
	return rs
}

// WithLoader configures a SensitiveEntityLoader that Warm uses to populate
// the sensitive-entity risk premium table. Call before Warm().
//
// Production usage in main.go:
//
//	scorer.WithLoader(func(ctx context.Context) ([]string, error) {
//	    rows, err := pool.Query(ctx, "SELECT entity_name FROM audit_sensitive_entities")
//	    ...
//	})
func (rs *RiskScorer) WithLoader(loader SensitiveEntityLoader) *RiskScorer {
	rs.loader = loader
	return rs
}

// Warm loads runtime scoring configuration by calling the registered
// SensitiveEntityLoader. It is called once during bootstrap after the
// database connection is available.
//
// If no loader is configured, or the loader returns nil, nil (e.g. because
// the audit_sensitive_entities table does not yet exist), Warm uses default
// rules and returns nil. This allows Phase 1 deployments to function before
// Phase 2 migrations are applied.
func (rs *RiskScorer) Warm(ctx context.Context) error {
	if rs.loader == nil {
		slog.InfoContext(ctx, "audit: RiskScorer using default rules (no loader configured)")
		return nil
	}
	entities, err := rs.loader(ctx)
	if err != nil {
		// Non-fatal: log and continue with defaults. Caller in main.go
		// also logs a warn-level message on non-nil error return.
		slog.WarnContext(ctx, "audit: RiskScorer loader failed; using default rules", "err", err)
		return err
	}
	if len(entities) == 0 {
		slog.InfoContext(ctx, "audit: RiskScorer warmed with default rules (no sensitive entities configured)")
		return nil
	}
	m := make(map[string]struct{}, len(entities))
	for _, e := range entities {
		m[e] = struct{}{}
	}
	next := &scorerState{sensitiveEntities: m}
	atomic.StorePointer(&rs.state, unsafe.Pointer(next))
	slog.InfoContext(ctx, "audit: RiskScorer warmed", "sensitive_entity_count", len(entities))
	return nil
}

// Score computes and returns the 0–100 risk score for the given record.
//
// Scoring rules (additive, capped at 100):
//
//	Base by operation:
//	  delete  → 30
//	  create  → 10
//	  update  → 10
//	  action  → 15
//	  login   → 5
//	  logout  → 0
//	  system  → 0
//
//	Category premium:
//	  ADMIN    → +30
//	  SECURITY → +30
//	  AUTH     → +10
//	  ACCESS   → +20
//	  other    → 0
//
//	Sensitive entity premium: +20
//
// The formula is intentionally simple for v1.0. A pluggable scoring engine
// is deferred to post-v1.0.
func (rs *RiskScorer) Score(record *AuditRecord) int {
	state := (*scorerState)(atomic.LoadPointer(&rs.state))

	score := operationBaseScore(record.Operation)
	score += categoryPremium(record.EventCategory)

	if _, sensitive := state.sensitiveEntities[record.EntityName]; sensitive {
		score += 20
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// severityFromScore maps a 0–100 risk score to a Severity level.
//
// Thresholds (AUDIT_ARCH.md §2.5):
//
//	≥70 → CRITICAL
//	≥50 → HIGH
//	≥30 → MEDIUM
//	≥10 → LOW
//	 <10 → INFO
func severityFromScore(score int) Severity {
	switch {
	case score >= 70:
		return SeverityCritical
	case score >= 50:
		return SeverityHigh
	case score >= 30:
		return SeverityMedium
	case score >= 10:
		return SeverityLow
	default:
		return SeverityInfo
	}
}

func operationBaseScore(op OperationType) int {
	switch op {
	case OperationDelete:
		return 30
	case OperationCreate:
		return 10
	case OperationUpdate:
		return 10
	case OperationAction:
		return 15
	case OperationLogin:
		return 5
	case OperationLogout:
		return 0
	case OperationSystem:
		return 0
	default:
		return 0
	}
}

func categoryPremium(cat EventCategory) int {
	switch cat {
	case CategoryAdmin:
		return 30
	case CategorySecurity:
		return 30
	case CategoryAccess:
		return 20
	case CategoryAuth:
		return 10
	default:
		return 0
	}
}

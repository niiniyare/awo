package audit

import (
	"fmt"
	"sort"

	"awo.so/awo/def"
)

// Sanitizer strips sensitive fields from entity snapshots before they are
// written into AuditRecord.BeforeData and AuditRecord.AfterData.
//
// Sensitive fields are determined by three sources (applied in order):
//  1. FieldDef.Sensitive == true in the EntityDefinition.
//  2. EntityAuditConfig.AdditionalSensitiveFields for the entity.
//  3. The runtime sensitive-field override set (populated from the
//     audit_sensitive_fields database table by RiskScorer.Warm).
//
// The Sanitizer is stateless for sources 1 and 2. Source 3 is injected via
// WithOverrides and updated by the RiskScorer after its Warm call completes.
type Sanitizer struct {
	// overrides holds additional sensitive field names loaded from the DB at
	// startup. Key: entityName; Value: set of field names.
	overrides map[string]map[string]struct{}
}

// NewSanitizer returns a Sanitizer with no runtime overrides. Overrides are
// added by calling WithOverrides after RiskScorer.Warm completes.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{overrides: make(map[string]map[string]struct{})}
}

// WithOverrides returns a new Sanitizer with the provided runtime overrides
// merged in. Existing overrides are replaced for affected entity names.
func (s *Sanitizer) WithOverrides(overrides map[string][]string) *Sanitizer {
	m := make(map[string]map[string]struct{}, len(s.overrides)+len(overrides))
	for k, v := range s.overrides {
		m[k] = v
	}
	for entity, fields := range overrides {
		set := make(map[string]struct{}, len(fields))
		for _, f := range fields {
			set[f] = struct{}{}
		}
		m[entity] = set
	}
	return &Sanitizer{overrides: m}
}

// Strip returns a copy of snapshot with all sensitive field values replaced by
// the redacted sentinel. The original map is never mutated.
//
// entityName is the qualified entity name (e.g. "finance_invoice"). snapshot
// may be nil (e.g. BeforeData on Create); Strip returns nil in that case.
func (s *Sanitizer) Strip(entityName string, snapshot map[string]any) map[string]any {
	if snapshot == nil {
		return nil
	}

	sensitive := s.sensitiveFields(entityName)
	if len(sensitive) == 0 {
		// No sensitive fields — return a shallow copy to protect the original.
		out := make(map[string]any, len(snapshot))
		for k, v := range snapshot {
			out[k] = v
		}
		return out
	}

	out := make(map[string]any, len(snapshot))
	for k, v := range snapshot {
		if _, redact := sensitive[k]; redact {
			out[k] = "[REDACTED]"
		} else {
			out[k] = v
		}
	}
	return out
}

// ComputeChangedFields returns a sorted list of field names whose values
// differ between before and after. Both maps must already be stripped.
// Returns nil when called for non-Update operations (before == nil or
// after == nil).
func ComputeChangedFields(before, after map[string]any) []string {
	if before == nil || after == nil {
		return nil
	}

	seen := make(map[string]struct{})

	// Fields present in before.
	for k := range before {
		seen[k] = struct{}{}
	}
	// Fields present in after.
	for k := range after {
		seen[k] = struct{}{}
	}

	var changed []string
	for k := range seen {
		bv, bOK := before[k]
		av, aOK := after[k]
		if !bOK || !aOK || !equalValues(bv, av) {
			changed = append(changed, k)
		}
	}

	sort.Strings(changed)
	return changed
}

// sensitiveFields returns the union of all sensitive field names for the
// entity: from its EntityDefinition, its EntityAuditConfig, and runtime DB
// overrides.
func (s *Sanitizer) sensitiveFields(entityName string) map[string]struct{} {
	var fields []string

	// Source 1: FieldDef.Sensitive in EntityDefinition.
	if ed := def.Lookup(entityName); ed != nil {
		for _, f := range ed.EntityFields() {
			if f.Sensitive {
				fields = append(fields, f.Name)
			}
		}
	}

	// Source 2: EntityAuditConfig.AdditionalSensitiveFields.
	cfg := ConfigFor(entityName)
	fields = append(fields, cfg.AdditionalSensitiveFields...)

	// Source 3: runtime DB overrides.
	if ov, ok := s.overrides[entityName]; ok {
		for f := range ov {
			fields = append(fields, f)
		}
	}

	if len(fields) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		set[f] = struct{}{}
	}
	return set
}

// equalValues performs a best-effort equality comparison for snapshot values.
// Uses fmt.Sprintf for types that do not support == (maps, slices).
func equalValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// For comparable primitive types use direct comparison.
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case int, int32, int64, float32, float64, bool:
		return a == b
	default:
		// Fall back to string representation for complex types (maps, slices).
		// This is intentionally conservative: any structural difference
		// (key ordering, nil vs empty slice) will mark the field as changed.
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

package audit

import (
	"strings"
	"testing"
)

func TestComputeChangedFields_Sorted(t *testing.T) {
	t.Parallel()

	before := map[string]any{"z_field": 1, "a_field": "x", "m_field": true}
	after := map[string]any{"z_field": 2, "a_field": "y", "m_field": true}

	changed := ComputeChangedFields(before, after)
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %d: %v", len(changed), changed)
	}
	// Must be sorted.
	if changed[0] > changed[1] {
		t.Errorf("ComputeChangedFields: result must be sorted ascending, got %v", changed)
	}
}

func TestComputeChangedFields_FieldAddedInAfter(t *testing.T) {
	t.Parallel()

	before := map[string]any{"name": "ACME"}
	after := map[string]any{"name": "ACME", "new_field": "value"}

	changed := ComputeChangedFields(before, after)
	if len(changed) != 1 || changed[0] != "new_field" {
		t.Errorf("expected [new_field], got %v", changed)
	}
}

func TestComputeChangedFields_FieldRemovedInAfter(t *testing.T) {
	t.Parallel()

	before := map[string]any{"name": "ACME", "old_field": "gone"}
	after := map[string]any{"name": "ACME"}

	changed := ComputeChangedFields(before, after)
	if len(changed) != 1 || changed[0] != "old_field" {
		t.Errorf("expected [old_field], got %v", changed)
	}
}

func TestComputeChangedFields_NilNilBothInputs(t *testing.T) {
	t.Parallel()

	if got := ComputeChangedFields(nil, nil); got != nil {
		t.Errorf("ComputeChangedFields(nil, nil) = %v, want nil", got)
	}
}

func TestComputeChangedFields_EmptyMaps(t *testing.T) {
	t.Parallel()

	got := ComputeChangedFields(map[string]any{}, map[string]any{})
	if len(got) != 0 {
		t.Errorf("ComputeChangedFields(empty, empty) = %v, want empty", got)
	}
}

func TestSanitizer_Strip_AllFieldsRedacted(t *testing.T) {
	t.Parallel()

	const entityName = "test_strip_all_fields"
	Register(EntityAuditConfig{
		EntityName:                entityName,
		Enabled:                   true,
		Category:                  CategoryData,
		Severity:                  SeverityInfo,
		AdditionalSensitiveFields: []string{"field_a", "field_b"},
	})

	s := NewSanitizer()
	input := map[string]any{
		"field_a": "secret_a",
		"field_b": "secret_b",
	}
	got := s.Strip(entityName, input)

	for k, v := range got {
		if v != "[REDACTED]" {
			t.Errorf("Strip: field %q should be [REDACTED], got %v", k, v)
		}
	}
}

func TestSanitizer_Strip_ReturnsCopyNotOriginal(t *testing.T) {
	t.Parallel()

	s := NewSanitizer()
	input := map[string]any{"name": "ACME"}
	got := s.Strip("no_sensitive_entity_xyz", input)

	// Mutate original — copy should not reflect change.
	input["name"] = "CHANGED"
	if got["name"] == "CHANGED" {
		t.Error("Strip must return a defensive copy, not a reference to the original map")
	}
}

func TestSanitizer_WithOverrides_DoesNotMutateOriginal(t *testing.T) {
	t.Parallel()

	orig := NewSanitizer()
	_ = orig.WithOverrides(map[string][]string{
		"some_entity": {"secret"},
	})

	// Original should not have the override.
	input := map[string]any{"secret": "value"}
	got := orig.Strip("some_entity", input)
	if v, ok := got["secret"]; ok && v == "[REDACTED]" {
		t.Error("WithOverrides must not mutate original Sanitizer")
	}
}

func TestEqualValues_AllTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b  any
		equal bool
	}{
		{nil, nil, true},
		{nil, "x", false},
		{"x", nil, false},
		{"hello", "hello", true},
		{"hello", "world", false},
		{int64(42), int64(42), true},
		{int64(42), int64(43), false},
		{true, true, true},
		{true, false, false},
		// Complex type falls back to fmt.Sprintf.
		{map[string]any{"k": "v"}, map[string]any{"k": "v"}, true},
	}

	for _, tc := range cases {
		got := equalValues(tc.a, tc.b)
		if got != tc.equal {
			t.Errorf("equalValues(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.equal)
		}
	}
}

func TestSanitizer_RedactedSentinelString(t *testing.T) {
	t.Parallel()

	const entityName = "test_redacted_sentinel"
	Register(EntityAuditConfig{
		EntityName:                entityName,
		Enabled:                   true,
		Category:                  CategoryData,
		Severity:                  SeverityInfo,
		AdditionalSensitiveFields: []string{"token"},
	})

	s := NewSanitizer()
	got := s.Strip(entityName, map[string]any{"token": "abc123"})
	if v, ok := got["token"]; !ok || v != "[REDACTED]" {
		t.Errorf("expected token = [REDACTED], got %v", v)
	}
	// Sentinel must not contain the original value.
	if strings.Contains("[REDACTED]", "abc123") {
		t.Error("redaction sentinel must not contain original value")
	}
}

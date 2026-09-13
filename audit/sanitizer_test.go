package audit

import (
	"testing"
)

func TestSanitizer_Strip_Nil(t *testing.T) {
	t.Parallel()

	s := NewSanitizer()
	if got := s.Strip("any_entity", nil); got != nil {
		t.Errorf("Strip(nil) = %v, want nil", got)
	}
}

func TestSanitizer_Strip_NoSensitiveFields(t *testing.T) {
	t.Parallel()

	s := NewSanitizer()
	input := map[string]any{"name": "ACME", "amount": 100}
	got := s.Strip("unknown_entity_xyz", input)

	if got["name"] != "ACME" {
		t.Errorf("Strip: non-sensitive field 'name' should not be redacted")
	}
	if got["amount"] != 100 {
		t.Errorf("Strip: non-sensitive field 'amount' should not be redacted")
	}
	// Must be a copy, not the same map.
	input["name"] = "MODIFIED"
	if got["name"] == "MODIFIED" {
		t.Error("Strip: returned map must be a copy")
	}
}

func TestSanitizer_Strip_WithAdditionalFields(t *testing.T) {
	t.Parallel()

	// Register entity with additional sensitive field.
	const entityName = "test_sanitizer_entity"
	Register(EntityAuditConfig{
		EntityName:                entityName,
		Enabled:                   true,
		Category:                  CategoryData,
		Severity:                  SeverityInfo,
		AdditionalSensitiveFields: []string{"secret_key"},
	})

	s := NewSanitizer()
	input := map[string]any{
		"name":       "ACME",
		"secret_key": "super-secret",
	}
	got := s.Strip(entityName, input)

	if got["name"] != "ACME" {
		t.Errorf("Strip: 'name' should not be redacted")
	}
	if got["secret_key"] != "[REDACTED]" {
		t.Errorf("Strip: 'secret_key' should be redacted, got %v", got["secret_key"])
	}
}

func TestSanitizer_WithOverrides(t *testing.T) {
	t.Parallel()

	s := NewSanitizer()
	s2 := s.WithOverrides(map[string][]string{
		"override_entity": {"password_hash"},
	})

	input := map[string]any{
		"email":         "user@example.com",
		"password_hash": "hash",
	}
	got := s2.Strip("override_entity", input)

	if got["email"] != "user@example.com" {
		t.Errorf("WithOverrides: 'email' should not be redacted")
	}
	if got["password_hash"] != "[REDACTED]" {
		t.Errorf("WithOverrides: 'password_hash' should be redacted")
	}

	// Original sanitizer must not be affected.
	got2 := s.Strip("override_entity", input)
	if got2["password_hash"] == "[REDACTED]" {
		t.Error("WithOverrides must not mutate original Sanitizer")
	}
}

func TestComputeChangedFields(t *testing.T) {
	t.Parallel()

	before := map[string]any{"name": "ACME", "status": "draft", "amount": 100}
	after := map[string]any{"name": "ACME", "status": "submitted", "amount": 200}

	changed := ComputeChangedFields(before, after)

	changedSet := make(map[string]struct{})
	for _, f := range changed {
		changedSet[f] = struct{}{}
	}

	if _, ok := changedSet["status"]; !ok {
		t.Error("ComputeChangedFields: 'status' should be in changed fields")
	}
	if _, ok := changedSet["amount"]; !ok {
		t.Error("ComputeChangedFields: 'amount' should be in changed fields")
	}
	if _, ok := changedSet["name"]; ok {
		t.Error("ComputeChangedFields: 'name' should not be in changed fields")
	}
}

func TestComputeChangedFields_NilInputs(t *testing.T) {
	t.Parallel()

	// Create operation: before is nil.
	if got := ComputeChangedFields(nil, map[string]any{"x": 1}); got != nil {
		t.Errorf("ComputeChangedFields(nil, after) = %v, want nil", got)
	}
	// Delete operation: after is nil.
	if got := ComputeChangedFields(map[string]any{"x": 1}, nil); got != nil {
		t.Errorf("ComputeChangedFields(before, nil) = %v, want nil", got)
	}
}

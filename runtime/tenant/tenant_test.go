package tenant_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/runtime/tenant"
)

func TestWithContext_FromContext_RoundTrip(t *testing.T) {
	id := uuid.New()
	tc := tenant.TenantContext{
		TenantID:   id,
		TenantSlug: "acme",
		Locale:     "en-KE",
		Timezone:   "Africa/Nairobi",
		Currency:   "KES",
	}
	ctx := tenant.WithContext(context.Background(), tc)
	got := tenant.FromContext(ctx)

	if got.TenantID != id {
		t.Errorf("TenantID mismatch: got %v, want %v", got.TenantID, id)
	}
	if got.TenantSlug != "acme" {
		t.Errorf("TenantSlug mismatch: got %q", got.TenantSlug)
	}
	if got.Locale != "en-KE" {
		t.Errorf("Locale mismatch: got %q", got.Locale)
	}
	if got.Timezone != "Africa/Nairobi" {
		t.Errorf("Timezone mismatch: got %q", got.Timezone)
	}
	if got.Currency != "KES" {
		t.Errorf("Currency mismatch: got %q", got.Currency)
	}
}

func TestFromContext_Panics_WhenAbsent(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "TenantContext not found") {
			t.Errorf("unexpected panic value: %v", r)
		}
	}()
	tenant.FromContext(context.Background())
}

func TestTryFromContext_ReturnsFalse_WhenAbsent(t *testing.T) {
	tc, ok := tenant.TryFromContext(context.Background())
	if ok {
		t.Error("expected ok=false, got true")
	}
	if tc.TenantID != uuid.Nil {
		t.Errorf("expected zero TenantContext, got %v", tc)
	}
}

func TestTryFromContext_ReturnsTrue_WhenPresent(t *testing.T) {
	id := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: id})
	tc, ok := tenant.TryFromContext(ctx)
	if !ok {
		t.Error("expected ok=true")
	}
	if tc.TenantID != id {
		t.Errorf("TenantID mismatch: got %v", tc.TenantID)
	}
}

func TestIDFromContext(t *testing.T) {
	id := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: id})
	got := tenant.IDFromContext(ctx)
	if got != id {
		t.Errorf("IDFromContext: got %v, want %v", got, id)
	}
}

func TestSystemContext(t *testing.T) {
	ctx := tenant.SystemContext(context.Background())
	tc := tenant.FromContext(ctx)
	if tc.TenantID != uuid.Nil {
		t.Errorf("SystemContext TenantID should be uuid.Nil, got %v", tc.TenantID)
	}
	if tc.Locale != "en-KE" {
		t.Errorf("SystemContext Locale should be en-KE, got %q", tc.Locale)
	}
}

func TestIsSystemContext_True(t *testing.T) {
	ctx := tenant.SystemContext(context.Background())
	if !tenant.IsSystemContext(ctx) {
		t.Error("expected IsSystemContext=true for SystemContext")
	}
}

func TestIsSystemContext_False_ForTenantContext(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: uuid.New()})
	if tenant.IsSystemContext(ctx) {
		t.Error("expected IsSystemContext=false for real tenant")
	}
}

func TestIsSystemContext_False_WhenAbsent(t *testing.T) {
	if tenant.IsSystemContext(context.Background()) {
		t.Error("expected IsSystemContext=false for empty context")
	}
}

func TestTenantContext_String(t *testing.T) {
	id := uuid.MustParse("12345678-1234-1234-1234-123456789012")
	tc := tenant.TenantContext{
		TenantID:   id,
		TenantSlug: "acme",
		Locale:     "en-KE",
	}
	s := tc.String()
	if !strings.Contains(s, "acme") {
		t.Errorf("String() should contain slug, got %q", s)
	}
	if !strings.Contains(s, "en-KE") {
		t.Errorf("String() should contain locale, got %q", s)
	}
}

func TestContextIsolation(t *testing.T) {
	// Two separate tenant contexts must not bleed into each other.
	id1 := uuid.New()
	id2 := uuid.New()

	ctx1 := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: id1})
	ctx2 := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: id2})

	tc1 := tenant.FromContext(ctx1)
	tc2 := tenant.FromContext(ctx2)

	if tc1.TenantID == tc2.TenantID {
		t.Error("separate tenant contexts should have different IDs")
	}
	if tc1.TenantID != id1 {
		t.Errorf("ctx1 has wrong tenant: got %v, want %v", tc1.TenantID, id1)
	}
	if tc2.TenantID != id2 {
		t.Errorf("ctx2 has wrong tenant: got %v, want %v", tc2.TenantID, id2)
	}
}

package tenant_test

import (
	"context"
	"testing"

	"awo.so/awo/def"
	"awo.so/awo/platform/tenant"
	"awo.so/awo/runtime"
)

func TestSlugValidator(t *testing.T) {
	v := &tenant.SlugValidator{}
	tests := []struct {
		slug    string
		wantErr bool
	}{
		{"acme-corp", false},
		{"my-company-123", false},
		{"ab", true},        // too short
		{"ACME", true},      // uppercase
		{"-acme", true},     // leading hyphen
		{"acme-", true},     // trailing hyphen
		{"acme corp", true}, // space
	}
	for _, tt := range tests {
		rec := &def.EntityRecord{Data: map[string]any{"slug": tt.slug}}
		err := v.BeforeCreate(context.Background(), rec)
		if (err != nil) != tt.wantErr {
			t.Errorf("slug %q: wantErr=%v, got err=%v", tt.slug, tt.wantErr, err)
		}
	}
}

func TestStatusValidator_EnforcesInitialPending(t *testing.T) {
	v := &tenant.StatusValidator{}

	// Empty status → should be set to PENDING.
	rec := &def.EntityRecord{Data: map[string]any{"status": ""}}
	if err := v.BeforeCreate(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error for empty status: %v", err)
	}
	if rec.GetString("status") != "PENDING" {
		t.Errorf("expected status=PENDING, got %q", rec.GetString("status"))
	}

	// Explicitly setting ACTIVE → rejected.
	rec2 := &def.EntityRecord{Data: map[string]any{"status": "ACTIVE"}}
	err := v.BeforeCreate(context.Background(), rec2)
	if err == nil {
		t.Fatal("expected error for ACTIVE initial status, got nil")
	}
	if !runtime.IsBusiness(err) {
		t.Errorf("expected BusinessError, got %T", err)
	}
}

func TestTransitionGuard(t *testing.T) {
	g := &tenant.TransitionGuard{}
	tests := []struct {
		from    string
		to      string
		wantErr bool
	}{
		{"PENDING", "ACTIVE", false},
		{"PENDING", "ARCHIVED", false},
		{"ACTIVE", "SUSPENDED", false},
		{"ACTIVE", "ARCHIVED", false},
		{"SUSPENDED", "ACTIVE", false},
		{"SUSPENDED", "ARCHIVED", false},
		{"ARCHIVED", "ACTIVE", true},   // terminal
		{"PENDING", "SUSPENDED", true}, // not allowed
		{"ACTIVE", "PENDING", true},    // not allowed
	}
	for _, tt := range tests {
		prev := &def.EntityRecord{Data: map[string]any{"status": tt.from}}
		rec := &def.EntityRecord{Data: map[string]any{"status": tt.to}}
		err := g.BeforeUpdate(context.Background(), rec, prev)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s→%s: wantErr=%v, got err=%v", tt.from, tt.to, tt.wantErr, err)
		}
	}
}

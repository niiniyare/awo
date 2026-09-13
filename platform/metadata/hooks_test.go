package metadata_test

import (
	"context"
	"testing"

	"awo.so/awo/def"
	"awo.so/awo/platform/metadata"
	"awo.so/awo/runtime"
)

func TestFieldNameValidator(t *testing.T) {
	v := &metadata.FieldNameValidator{}
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"cf_invoice_ref", false},
		{"cf_x", false},
		{"invoice_ref", true}, // missing cf_ prefix
		{"CF_invoice", true},  // uppercase
		{"cf_", true},         // just prefix, no name
		{"cf_123", true},      // starts with digit after cf_
		{"cf_good_name_123", false},
	}
	for _, tt := range tests {
		rec := &def.EntityRecord{Data: map[string]any{"field_name": tt.name}}
		err := v.BeforeCreate(context.Background(), rec)
		if (err != nil) != tt.wantErr {
			t.Errorf("name %q: wantErr=%v, got err=%v", tt.name, tt.wantErr, err)
		}
		if err != nil && !runtime.IsValidation(err) {
			t.Errorf("name %q: expected ValidationError, got %T", tt.name, err)
		}
	}
}

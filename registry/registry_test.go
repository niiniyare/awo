package registry_test

import (
	"regexp"
	"testing"

	"awo.so/awo/def"
)

// entityNameRe mirrors the internal regexp used by the registry validator.
var entityNameRe = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z][a-z0-9]*)+$`)

func TestEntityNameFormat(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"finance_invoice", true},
		{"finance_invoice_line", true},
		{"hr_employee", true},
		{"inventory_stock_move", true},
		{"Invoice", false},          // PascalCase
		{"finance-invoice", false},  // hyphen
		{"finance", false},          // no underscore separator
		{"_finance_invoice", false}, // leading underscore
		{"finance_", false},         // trailing underscore
		{"FINANCE_INVOICE", false},  // uppercase
	}

	for _, tc := range cases {
		got := entityNameRe.MatchString(tc.name)
		if got != tc.valid {
			t.Errorf("name %q: expected valid=%v, got %v", tc.name, tc.valid, got)
		}
	}
}

// TestRegisterAndLookup exercises the def package registration that the
// registry reads. Run with -count=1 to avoid cross-test pollution on the
// global registry.
func TestDefRegisterAndLookup(t *testing.T) {
	const entityName = "registrytest_widget"

	// Skip if already registered by a parallel test in the same binary.
	if def.Lookup(entityName) != nil {
		t.Skipf("entity %q already registered; skipping to avoid duplicate panic", entityName)
	}

	d := &def.SystemDefinition{
		Name:        entityName,
		Module:      "registrytest",
		Label:       "Widget",
		LabelPlural: "Widgets",
	}

	def.Register(d)

	found := def.Lookup(entityName)
	if found == nil {
		t.Fatalf("Lookup(%q) returned nil after Register", entityName)
	}
	if found.EntityName() != entityName {
		t.Errorf("EntityName: got %q, want %q", found.EntityName(), entityName)
	}
	if found.IsSystem() != true {
		t.Error("IsSystem: expected true for SystemDefinition")
	}
}

func TestDefCount_IncreasesAfterRegister(t *testing.T) {
	before := def.Count()

	const entityName = "registrytest_counter"
	if def.Lookup(entityName) != nil {
		t.Skipf("entity %q already registered", entityName)
	}

	def.Register(&def.SystemDefinition{
		Name:        entityName,
		Module:      "registrytest",
		Label:       "Counter",
		LabelPlural: "Counters",
	})

	after := def.Count()
	if after != before+1 {
		t.Errorf("Count: expected %d, got %d", before+1, after)
	}
}

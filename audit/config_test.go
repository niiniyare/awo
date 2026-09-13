package audit

import (
	"testing"
)

func TestConfigFor_Default(t *testing.T) {
	t.Parallel()

	// Entity with no explicit registration gets default config.
	cfg := ConfigFor("nonexistent_entity")
	if !cfg.Enabled {
		t.Error("default config: Enabled must be true")
	}
	if cfg.Category != CategoryData {
		t.Errorf("default config: Category = %v, want %v", cfg.Category, CategoryData)
	}
	if cfg.Severity != SeverityInfo {
		t.Errorf("default config: Severity = %v, want %v", cfg.Severity, SeverityInfo)
	}
	if cfg.EntityName != "nonexistent_entity" {
		t.Errorf("default config: EntityName = %v, want nonexistent_entity", cfg.EntityName)
	}
}

func TestRegister_PanicsOnEmptyName(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Register with empty EntityName should panic")
		}
	}()
	Register(EntityAuditConfig{EntityName: ""})
}

func TestRegister_And_ConfigFor(t *testing.T) {
	// Not parallel — mutates registry global.
	// Use a unique name to avoid collision with other tests.
	const entityName = "test_register_entity_audit"

	Register(EntityAuditConfig{
		EntityName: entityName,
		Enabled:    false,
		Category:   CategoryAdmin,
		Severity:   SeverityCritical,
	})

	cfg := ConfigFor(entityName)
	if cfg.Enabled {
		t.Error("registered config: Enabled should be false")
	}
	if cfg.Category != CategoryAdmin {
		t.Errorf("registered config: Category = %v, want %v", cfg.Category, CategoryAdmin)
	}
	if cfg.Severity != SeverityCritical {
		t.Errorf("registered config: Severity = %v, want %v", cfg.Severity, SeverityCritical)
	}
}

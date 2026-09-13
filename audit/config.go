package audit

import (
	"sync"
)

// EntityAuditConfig declares the audit behaviour for a single entity.
// Instances are registered once per entity during init() via Register.
//
// Entities without an explicit registration use defaultConfig.
type EntityAuditConfig struct {
	// EntityName is the qualified entity name (e.g. "finance_invoice").
	// Must match the value returned by EntityDefinition.EntityName().
	EntityName string

	// Enabled controls whether audit records are written for this entity.
	// Default: true.
	Enabled bool

	// Category overrides the default EventCategory for DATA-class entities.
	// Entities in IAM or platform modules may declare CategoryAdmin or
	// CategorySecurity here. Default: CategoryData.
	Category EventCategory

	// Severity is the default severity applied when no explicit severity is
	// set by the operation. Default: SeverityInfo.
	Severity Severity

	// AdditionalSensitiveFields lists field names (beyond those declared
	// Sensitive:true in their FieldDef) that must be stripped from
	// BeforeData/AfterData snapshots. Field names are matched case-sensitively
	// against snapshot map keys.
	AdditionalSensitiveFields []string

	// ComplianceFlags declares which compliance frameworks apply to this
	// entity. These are merged into AuditRecord.ComplianceFlags on every
	// record emitted for the entity.
	//
	// Example: map[string]bool{"GDPR": true, "KRA_ETIMS": true}
	ComplianceFlags map[string]bool
}

// defaultConfig is returned for entities that have not called Register.
var defaultConfig = EntityAuditConfig{
	Enabled:  true,
	Category: CategoryData,
	Severity: SeverityInfo,
}

// registry holds entity-name → EntityAuditConfig entries.
var registry struct {
	mu     sync.RWMutex
	sealed bool
	m      map[string]EntityAuditConfig
}

func init() {
	registry.m = make(map[string]EntityAuditConfig)
}

// Register declares audit configuration for a single entity. Must be called
// from an init() function — never from a handler, service, or after bootstrap.
//
// Panics if called after Seal() or if EntityName is empty.
func Register(cfg EntityAuditConfig) {
	if cfg.EntityName == "" {
		panic("audit.Register: EntityName must not be empty")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if registry.sealed {
		panic("audit.Register: registry is sealed; Register may only be called from init()")
	}

	registry.m[cfg.EntityName] = cfg
}

// ConfigFor returns the EntityAuditConfig for the named entity, or the
// default config if no registration exists. Safe to call from any goroutine
// after Seal() has been called.
func ConfigFor(entityName string) EntityAuditConfig {
	registry.mu.RLock()
	cfg, ok := registry.m[entityName]
	registry.mu.RUnlock()

	if !ok {
		c := defaultConfig
		c.EntityName = entityName
		return c
	}
	return cfg
}

// Seal marks the registry as immutable. Called once by bootstrap after all
// init() functions have run. Any call to Register after Seal panics.
func Seal() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.sealed = true
}

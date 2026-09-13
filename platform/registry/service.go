package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
)

// Service manages module installation and activation records.
//
// The service tracks which modules are installed platform-wide (platform_module)
// and which are activated per tenant (platform_tenant_module). It does not
// execute provisioning workflows — callers wire WorkflowTriggers on the
// platform_tenant_module entity to launch Temporal workflows for seeding.
type Service struct {
	modules       driver.EntityRepository[*def.EntityRecord]
	tenantModules driver.EntityRepository[*def.EntityRecord]
}

// NewService creates a module registry service.
func NewService(
	modules driver.EntityRepository[*def.EntityRecord],
	tenantModules driver.EntityRepository[*def.EntityRecord],
) *Service {
	return &Service{modules: modules, tenantModules: tenantModules}
}

// RegisterModule registers a module as available on the platform.
// Idempotent — if the module key already exists, returns the existing record.
func (s *Service) RegisterModule(ctx context.Context, key, label, version string) (*def.EntityRecord, error) {
	existing, _, err := s.modules.Query(ctx, filter.Eq("key", key), driver.WithSkipCount())
	if err != nil {
		return nil, fmt.Errorf("registry.RegisterModule: query %q: %w", key, err)
	}
	if len(existing) > 0 {
		return existing[0], nil
	}
	rec, err := s.modules.Create(ctx, driver.CreateInput{
		Data: map[string]any{
			"key":     key,
			"label":   label,
			"version": version,
			"active":  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("registry.RegisterModule: create %q: %w", key, err)
	}
	return rec, nil
}

// Activate activates a module for a tenant.
// Creates a platform_tenant_module record in status="installing".
// A WorkflowTrigger on the entity fires the provisioning workflow.
func (s *Service) Activate(ctx context.Context, tenantID uuid.UUID, moduleKey string) (*def.EntityRecord, error) {
	existing, _, err := s.tenantModules.Query(ctx, filter.And(
		filter.Eq("module_key", moduleKey),
	), driver.WithSkipCount())
	if err != nil {
		return nil, fmt.Errorf("registry.Activate: query %q: %w", moduleKey, err)
	}
	if len(existing) > 0 {
		status := existing[0].GetString("status")
		if status == "active" {
			return existing[0], nil
		}
		if status == "suspended" {
			rec, err := s.tenantModules.Update(ctx, existing[0].ID, driver.UpdateInput{
				Data: map[string]any{"status": "active"},
			})
			if err != nil {
				return nil, fmt.Errorf("registry.Activate: re-enable %q: %w", moduleKey, err)
			}
			return rec, nil
		}
		return nil, &runtime.BusinessError{
			Code:    "module.already_activating",
			Message: fmt.Sprintf("module %q is currently %s", moduleKey, status),
			Status:  409,
		}
	}

	rec, err := s.tenantModules.Create(ctx, driver.CreateInput{
		Data: map[string]any{
			"module_key":   moduleKey,
			"status":       "installing",
			"installed_at": time.Now().UTC(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("registry.Activate: create tenant_module: %w", err)
	}
	return rec, nil
}

// Disable suspends a module for a tenant without uninstalling it.
func (s *Service) Disable(ctx context.Context, moduleKey string) error {
	records, _, err := s.tenantModules.Query(ctx, filter.Eq("module_key", moduleKey), driver.WithSkipCount())
	if err != nil {
		return fmt.Errorf("registry.Disable: query %q: %w", moduleKey, err)
	}
	if len(records) == 0 {
		return &runtime.NotFoundError{EntityName: "platform_tenant_module", ID: moduleKey}
	}
	_, err = s.tenantModules.Update(ctx, records[0].ID, driver.UpdateInput{
		Data: map[string]any{"status": "suspended"},
	})
	if err != nil {
		return fmt.Errorf("registry.Disable: update %q: %w", moduleKey, err)
	}
	return nil
}

// ListActive returns all active modules for the current tenant (via RLS context).
func (s *Service) ListActive(ctx context.Context) ([]*def.EntityRecord, error) {
	records, _, err := s.tenantModules.Query(ctx, filter.Eq("status", "active"), driver.WithSkipCount())
	if err != nil {
		return nil, fmt.Errorf("registry.ListActive: %w", err)
	}
	return records, nil
}

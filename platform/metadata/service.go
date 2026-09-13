package metadata

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
)

// Service manages the runtime custom-field schema for all entities.
//
// Changes to custom fields take effect on the next request — the schema is
// not cached at the service layer (SDUI caches the rendered page schema with
// a 5-minute TTL, which is invalidated separately by the SDUI builder).
type Service struct {
	repo driver.EntityRepository[*def.EntityRecord]
}

// NewService creates a metadata service backed by the given repository.
func NewService(repo driver.EntityRepository[*def.EntityRecord]) *Service {
	return &Service{repo: repo}
}

// FieldsForEntity returns all active custom field definitions for the named entity.
// Results include all tenants' fields — callers must filter by tenant_id if needed.
func (s *Service) FieldsForEntity(ctx context.Context, entityName string) ([]*def.EntityRecord, error) {
	f := filter.And(
		filter.Eq("entity_name", entityName),
		filter.Eq("active", true),
	)
	records, _, err := s.repo.Query(ctx, f, driver.WithSkipCount(), driver.WithSort("sort_order", true))
	if err != nil {
		return nil, fmt.Errorf("metadata.FieldsForEntity %q: %w", entityName, err)
	}
	return records, nil
}

// AddField creates a new custom field definition on an entity.
//
// Field names must follow the cf_ prefix convention (enforced by FieldNameValidator hook).
func (s *Service) AddField(ctx context.Context, tenantID uuid.UUID, entityName, fieldName, fieldType, label string) (*def.EntityRecord, error) {
	rec, err := s.repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{
			"entity_name": entityName,
			"field_name":  fieldName,
			"field_type":  fieldType,
			"label":       label,
			"active":      true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("metadata.AddField: create field %q on %q: %w", fieldName, entityName, err)
	}
	return rec, nil
}

// DeactivateField marks a custom field as inactive. The field data remains in
// the JSONB column but is excluded from API responses and SDUI pages.
//
// Fields are never hard-deleted — deactivation is the only safe removal path.
func (s *Service) DeactivateField(ctx context.Context, fieldID uuid.UUID) error {
	_, err := s.repo.Update(ctx, fieldID, driver.UpdateInput{
		Data: map[string]any{"active": false},
	})
	if err != nil {
		return fmt.Errorf("metadata.DeactivateField %s: %w", fieldID, err)
	}
	return nil
}

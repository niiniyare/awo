package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/cache"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
)

const (
	settingCacheTTL    = 5 * time.Minute
	settingCachePrefix = "setting:"
)

// Service resolves setting values using scope-based hierarchy.
type Service struct {
	repo  driver.EntityRepository[*def.EntityRecord]
	cache cache.Cache
}

// NewService creates a settings service.
func NewService(repo driver.EntityRepository[*def.EntityRecord], c cache.Cache) *Service {
	return &Service{repo: repo, cache: c}
}

// Get retrieves the effective value for namespace+key, resolving scope hierarchy.
// Scope priority: branch → tenant → system.
// scopeRef is the UUID of the branch or tenant (empty string for system scope).
func (s *Service) Get(ctx context.Context, namespace, key string, tenantID uuid.UUID) (any, error) {
	cacheKey := fmt.Sprintf("%s%s.%s.%s", settingCachePrefix, namespace, key, tenantID)
	var result any
	if err := s.cache.Get(ctx, cacheKey, &result); err == nil {
		return result, nil
	}

	val, err := s.resolve(ctx, namespace, key, tenantID)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, cacheKey, val, settingCacheTTL)
	return val, nil
}

// GetString is a typed helper for string settings.
func (s *Service) GetString(ctx context.Context, namespace, key string, tenantID uuid.UUID, def string) string {
	v, err := s.Get(ctx, namespace, key, tenantID)
	if err != nil || v == nil {
		return def
	}
	if str, ok := v.(string); ok {
		return str
	}
	return def
}

// Set writes or updates a setting value for the given scope.
func (s *Service) Set(ctx context.Context, namespace, key, scope, scopeRef string, value any) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("settings.Set: marshal value: %w", err)
	}

	// Check if setting already exists for this scope.
	f := filter.And(
		filter.Eq("namespace", namespace),
		filter.Eq("key", key),
		filter.Eq("scope", scope),
		filter.Eq("scope_ref", scopeRef),
	)
	existing, _, err := s.repo.Query(ctx, f, driver.WithSkipCount())
	if err != nil {
		return fmt.Errorf("settings.Set: query: %w", err)
	}

	if len(existing) > 0 {
		_, err = s.repo.Update(ctx, existing[0].ID, driver.UpdateInput{
			Data: map[string]any{"value": valueJSON},
		})
	} else {
		_, err = s.repo.Create(ctx, driver.CreateInput{
			Data: map[string]any{
				"namespace": namespace,
				"key":       key,
				"scope":     scope,
				"scope_ref": scopeRef,
				"value":     valueJSON,
			},
		})
	}
	if err != nil {
		return fmt.Errorf("settings.Set: persist: %w", err)
	}

	// Invalidate cache for this namespace+key across all tenants.
	_ = s.cache.DeletePrefix(ctx, settingCachePrefix+namespace+"."+key)
	return nil
}

func (s *Service) resolve(ctx context.Context, namespace, key string, tenantID uuid.UUID) (any, error) {
	// Query all scopes — tenant-scoped query via RLS returns only tenant records.
	results, _, err := s.repo.Query(ctx, filter.And(
		filter.Eq("namespace", namespace),
		filter.Eq("key", key),
	), driver.WithSkipCount())
	if err != nil {
		return nil, fmt.Errorf("settings.resolve: %w", err)
	}

	// Priority: branch scope first (scope_ref matches branch UUID),
	// then tenant scope, then system.
	scopePriority := map[string]int{"branch": 3, "tenant": 2, "system": 1}
	var best *def.EntityRecord
	bestPriority := 0
	for _, r := range results {
		scope := r.GetString("scope")
		p := scopePriority[scope]
		if p > bestPriority {
			bestPriority = p
			rec := r
			best = rec
		}
	}
	if best == nil {
		return nil, nil
	}

	raw := best.Get("value")
	if raw == nil {
		return nil, nil
	}
	// value is stored as JSON bytes — unmarshal to native type.
	var parsed any
	switch v := raw.(type) {
	case []byte:
		if err := json.Unmarshal(v, &parsed); err != nil {
			return raw, nil
		}
	case string:
		if err := json.Unmarshal([]byte(v), &parsed); err != nil {
			return v, nil
		}
	default:
		parsed = v
	}
	return parsed, nil
}

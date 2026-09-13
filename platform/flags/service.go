package flags

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/cache"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
)

const (
	flagCacheTTL    = 5 * time.Minute
	flagCachePrefix = "eval:"
)

// Service evaluates feature flags against cached results.
type Service struct {
	flags     driver.EntityRepository[*def.EntityRecord]
	overrides driver.EntityRepository[*def.EntityRecord]
	cache     cache.Cache
}

// NewService creates a flag evaluation service.
func NewService(
	flags driver.EntityRepository[*def.EntityRecord],
	overrides driver.EntityRepository[*def.EntityRecord],
	c cache.Cache,
) *Service {
	return &Service{flags: flags, overrides: overrides, cache: c}
}

// IsEnabled reports whether the flag is enabled for the given tenant and user.
// Evaluation order: user override (future) → tenant override → system default.
// Results are cached for flagCacheTTL.
func (s *Service) IsEnabled(ctx context.Context, key string, tenantID, userID uuid.UUID) (bool, error) {
	cacheKey := evalCacheKey(key, tenantID, userID)

	var result bool
	if err := s.cache.Get(ctx, cacheKey, &result); err == nil {
		return result, nil
	}

	enabled, err := s.evaluate(ctx, key, tenantID)
	if err != nil {
		return false, err
	}

	_ = s.cache.Set(ctx, cacheKey, enabled, flagCacheTTL)
	return enabled, nil
}

// Invalidate clears all cached evaluations for the given flag key.
// Called when a flag or override is changed.
func (s *Service) Invalidate(ctx context.Context, key string) error {
	prefix := flagCachePrefix + sha256prefix(key)
	return s.cache.DeletePrefix(ctx, prefix)
}

func (s *Service) evaluate(ctx context.Context, key string, tenantID uuid.UUID) (bool, error) {
	// Load the system flag.
	results, _, err := s.flags.Query(ctx, filter.Eq("key", key), driver.WithSkipCount())
	if err != nil {
		return false, fmt.Errorf("flags.evaluate: query flag %q: %w", key, err)
	}
	if len(results) == 0 {
		return false, nil // unknown flag → disabled
	}
	flag := results[0]
	systemEnabled, _ := flag.Get("enabled").(bool)

	// Check tenant override.
	overrides, _, err := s.overrides.Query(ctx, filter.And(
		filter.Eq("flag_id", flag.ID),
	), driver.WithSkipCount())
	if err != nil {
		return false, fmt.Errorf("flags.evaluate: query overrides: %w", err)
	}
	for _, ov := range overrides {
		// The override record is tenant-scoped via RLS — first result wins.
		if tenantEnabled, ok := ov.Get("enabled").(bool); ok {
			return tenantEnabled, nil
		}
	}

	return systemEnabled, nil
}

func evalCacheKey(flagKey string, tenantID, userID uuid.UUID) string {
	h := sha256.New()
	h.Write([]byte(flagKey))
	h.Write([]byte(tenantID.String()))
	h.Write([]byte(userID.String()))
	return fmt.Sprintf("%s%x", flagCachePrefix, h.Sum(nil))
}

func sha256prefix(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:4]) // 4-byte prefix for namespace grouping
}

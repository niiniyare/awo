package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
)

// AsActionRepo adapts s into a def.ActionEntityRepo bound to actor, for use
// as runtime.ActionContext's per-entity Repo(entityName) result (Phase 2
// Step 4). Every method call goes through the exact same canonical mutation
// pipeline (validation, hooks, audit, durable outbox event — ADR-025 §5)
// that an ordinary HTTP request against this entity uses: custom actions do
// not get a second, lesser mutation path. Create/Update/Delete are
// attributed to actor, matching how the HTTP handlers attribute mutations to
// the authenticated caller.
func (s *EntityService) AsActionRepo(actor *def.Actor) def.ActionEntityRepo {
	return &entityServiceActionRepo{svc: s, actor: actor}
}

type entityServiceActionRepo struct {
	svc   *EntityService
	actor *def.Actor
}

var _ def.ActionEntityRepo = (*entityServiceActionRepo)(nil)

func (r *entityServiceActionRepo) EntityName() string {
	return r.svc.schema.QualifiedName
}

func (r *entityServiceActionRepo) Get(ctx context.Context, id uuid.UUID) (*def.EntityRecord, error) {
	return r.svc.Get(ctx, id)
}

// Query converts the ActionFilter/ActionQueryOpt surface into the
// repository's *filter.Filter/driver.QueryOption surface.
//
// Known, disclosed limitation: driver.QueryOptions only supports page-based
// pagination (1-based Page + PageSize — driver.WithPage), while
// def.ActionQueryConfig is offset-based (Limit + Offset). A non-page-aligned
// Offset (one that is not an exact multiple of Limit) cannot be expressed
// without either silently returning the wrong page or fabricating a
// resumable-offset mechanism the driver does not have — this method refuses
// the request with a clear error in that case rather than silently
// returning incorrect data. No known caller (the one real ActionDef,
// platform/notification's mark_read, does not call Query at all) is
// affected by this today.
func (r *entityServiceActionRepo) Query(ctx context.Context, f def.ActionFilter, opts ...def.ActionQueryOpt) ([]*def.EntityRecord, error) {
	cfg := &def.ActionQueryConfig{}
	for _, o := range opts {
		o(cfg)
	}
	qopts, err := actionQueryOptions(cfg)
	if err != nil {
		return nil, err
	}
	records, _, err := r.svc.Query(ctx, actionFilterValue(f), qopts...)
	return records, err
}

func (r *entityServiceActionRepo) Count(ctx context.Context, f def.ActionFilter) (int64, error) {
	return r.svc.Count(ctx, actionFilterValue(f))
}

func (r *entityServiceActionRepo) Exists(ctx context.Context, f def.ActionFilter) (bool, error) {
	return r.svc.Exists(ctx, actionFilterValue(f))
}

func (r *entityServiceActionRepo) Create(ctx context.Context, data map[string]any) (*def.EntityRecord, error) {
	return r.svc.Create(ctx, data, r.actor)
}

func (r *entityServiceActionRepo) Update(ctx context.Context, id uuid.UUID, patch map[string]any) (*def.EntityRecord, error) {
	return r.svc.Update(ctx, id, patch, r.actor)
}

func (r *entityServiceActionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.svc.Delete(ctx, id, r.actor)
}

// actionFilterValue extracts the *filter.Filter from an ActionFilter (any),
// per def.ActionFilter's own documented contract ("the concrete type is
// *filter.Filter... Do not implement this with types outside
// awo.so/awo/filter"). A nil or wrongly-typed value yields a nil filter
// (matching EntityService.Query's own "nil filter" convention), not a panic.
func actionFilterValue(f def.ActionFilter) *filter.Filter {
	ff, _ := f.(*filter.Filter)
	return ff
}

// actionQueryOptions converts a def.ActionQueryConfig into driver.QueryOptions.
// See Query's doc comment for the one disclosed pagination limitation.
func actionQueryOptions(cfg *def.ActionQueryConfig) ([]driver.QueryOption, error) {
	var opts []driver.QueryOption

	if cfg.Order != "" {
		field, asc := cfg.Order, true
		upper := strings.ToUpper(cfg.Order)
		switch {
		case strings.HasSuffix(upper, " DESC"):
			field, asc = strings.TrimSpace(cfg.Order[:len(cfg.Order)-len(" DESC")]), false
		case strings.HasSuffix(upper, " ASC"):
			field = strings.TrimSpace(cfg.Order[:len(cfg.Order)-len(" ASC")])
		}
		opts = append(opts, driver.WithSort(field, asc))
	}

	if cfg.Limit > 0 {
		page := 1
		if cfg.Offset > 0 {
			if cfg.Offset%cfg.Limit != 0 {
				return nil, fmt.Errorf(
					"api/service: ActionEntityRepo.Query: offset %d is not a multiple of limit %d — only page-aligned offsets are supported",
					cfg.Offset, cfg.Limit)
			}
			page = cfg.Offset/cfg.Limit + 1
		}
		opts = append(opts, driver.WithPage(page, cfg.Limit))
	}

	return opts, nil
}

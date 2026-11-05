package pgx

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LinkResolver implements driver.LinkLabelResolver against PostgreSQL.
//
// Tenant isolation: resolving a label is still a read of a real,
// RLS-protected table (e.g. iam_user), so it goes through the exact same
// withTenantReadTx path Repository.Get/Query use — reusing the caller's
// existing transaction if there is one, otherwise opening a short-lived read
// transaction and setting the "awo.tenant_id" GUC from the request's
// authenticated tenant.TenantContext before the query runs. An earlier
// version of this file queried the bare pool directly and merely claimed RLS
// "was already active on the connection" — that was never verified and was
// false: an autocommit pool.Query has no transaction for set_tenant_context's
// set_config(..., true) to scope to, so current_tenant_id() reads NULL and
// RLS fails closed (see link_resolver_rls_test.go, which proves this against
// the real non-superuser awo_app role — a superuser connection, which this
// demo's admin role happens to be, bypasses RLS entirely and would never
// have caught this).
// customEntityRecordsTable is the one physical table every custom (JSONB)
// entity shares — see compiler.buildEntitySchema, which sets this exact
// TableName whenever !IsSystem. A custom entity's own fields live inside
// that table's single "data" jsonb column rather than as top-level SQL
// columns, which ResolveLinkLabels below must account for.
const customEntityRecordsTable = "custom_entity_records"

type LinkResolver struct {
	pool *pgxpool.Pool
}

// NewLinkResolver creates a LinkResolver backed by pool.
func NewLinkResolver(pool *pgxpool.Pool) *LinkResolver {
	return &LinkResolver{pool: pool}
}

// ResolveLinkLabels looks up labelField for each row in targetTable whose id
// is in ids. targetTable and labelField come from compile-time schema
// metadata (compiler.CompiledLookup / EntitySchema.TableName), never from
// request input, so direct interpolation into the query text follows the
// same convention already used throughout this package (e.g. Repository's
// own `SELECT ... FROM "<table>"` calls in repo.go).
func (r *LinkResolver) ResolveLinkLabels(ctx context.Context, targetTable, labelField string, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	// A link into a custom entity still resolves against targetTable =
	// customEntityRecordsTable (compile-time schema metadata), but labelField
	// names a key inside that table's jsonb "data" column, not a real SQL
	// column — interpolating it as a bare identifier like the system-entity
	// case would produce "column \"<labelField>\" does not exist" for every
	// such lookup.
	labelExpr := fmt.Sprintf(`"%s"`, labelField)
	if targetTable == customEntityRecordsTable {
		labelExpr = fmt.Sprintf(`"data"->>'%s'`, labelField)
	}

	err := withTenantReadTx(ctx, r.pool, func(txCtx context.Context) error {
		conn := connFromContext(txCtx, r.pool)
		sql := fmt.Sprintf(`SELECT "id", %s FROM "%s" WHERE "id" = ANY($1)`, labelExpr, targetTable)
		rows, err := conn.db().Query(txCtx, sql, ids)
		if err != nil {
			return fmt.Errorf("pgx.LinkResolver: query %q: %w", targetTable, err)
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			var label *string
			if err := rows.Scan(&id, &label); err != nil {
				return fmt.Errorf("pgx.LinkResolver: scan %q: %w", targetTable, err)
			}
			if label != nil {
				out[id] = *label
			}
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

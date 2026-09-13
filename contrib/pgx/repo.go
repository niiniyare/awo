package pgx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	pgxlib "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"awo.so/awo/compiler"
	"awo.so/awo/contrib/pgx/sqlbuild"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/internal/dberr"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	"awo.so/awo/tx"
)

// Repository implements driver.EntityRepository[*def.EntityRecord] for
// PostgreSQL via pgx v5.
//
// TenantID is always sourced from the TenantContext in ctx — the driver never
// accepts it as a parameter. RLS enforces tenant isolation at the database
// level; this is a defence-in-depth measure.
type Repository struct {
	pool   *pgxpool.Pool
	schema *compiler.EntitySchema
}

// NewRepository creates a Repository for the given entity schema.
func NewRepository(pool *pgxpool.Pool, schema *compiler.EntitySchema) *Repository {
	return &Repository{pool: pool, schema: schema}
}

// Ensure compile-time interface satisfaction.
var _ driver.EntityRepository[*def.EntityRecord] = (*Repository)(nil)

// -------------------------------------------------------------------------
// Read operations
// -------------------------------------------------------------------------

// Get fetches a single record by primary key.
func (r *Repository) Get(ctx context.Context, id uuid.UUID, opts ...driver.QueryOption) (*def.EntityRecord, error) {
	qo := driver.ResolveOptions(opts)
	conn := connFromContext(ctx, r.pool)

	cols, scan := r.columnsAndScanner()
	sql := fmt.Sprintf(
		`SELECT %s FROM "%s" WHERE "id" = $1 LIMIT 1`,
		cols, r.schema.TableName,
	)
	if qo.ForUpdate {
		sql += " FOR UPDATE"
	}

	rec, err := scan(conn.db().QueryRow(ctx, sql, id))
	if err != nil {
		if err == pgxlib.ErrNoRows {
			return nil, &runtime.NotFoundError{EntityName: r.schema.TableName, ID: id.String()}
		}
		return nil, dberr.Parse(err, r.schema.TableName+".Get")
	}
	return rec, nil
}

// Query returns records matching f with pagination and sorting.
func (r *Repository) Query(ctx context.Context, f *filter.Filter, opts ...driver.QueryOption) ([]*def.EntityRecord, driver.PageInfo, error) {
	qo := driver.ResolveOptions(opts)
	conn := connFromContext(ctx, r.pool)

	where, err := sqlbuild.Build(f, 0)
	if err != nil {
		return nil, driver.PageInfo{}, fmt.Errorf("%s.Query: build filter: %w", r.schema.TableName, err)
	}

	cols, scan := r.columnsAndScanner()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`SELECT %s FROM "%s"`, cols, r.schema.TableName))
	if where.Clause != "" {
		sb.WriteString(` WHERE ` + where.Clause)
	}
	if qo.SortField != "" {
		dir := "DESC"
		if qo.SortAsc {
			dir = "ASC"
		}
		sb.WriteString(fmt.Sprintf(` ORDER BY "%s" %s`, qo.SortField, dir))
	}
	pageSize := qo.PageSize
	offset := 0
	if qo.Page > 1 {
		offset = (qo.Page - 1) * pageSize
	}
	// Fetch pageSize+1 to detect HasNextPage without a separate COUNT.
	sb.WriteString(fmt.Sprintf(` LIMIT %d OFFSET %d`, pageSize+1, offset))

	rows, err := conn.db().Query(ctx, sb.String(), where.Args...)
	if err != nil {
		return nil, driver.PageInfo{}, dberr.Parse(err, r.schema.TableName+".Query")
	}
	defer rows.Close()

	var records []*def.EntityRecord
	for rows.Next() {
		rec, err := scan(rows)
		if err != nil {
			return nil, driver.PageInfo{}, dberr.Parse(err, r.schema.TableName+".Query.scan")
		}
		records = append(records, rec)
	}
	if rows.Err() != nil {
		return nil, driver.PageInfo{}, dberr.Parse(rows.Err(), r.schema.TableName+".Query.rows")
	}

	hasNext := len(records) > pageSize
	if hasNext {
		records = records[:pageSize]
	}
	info := driver.PageInfo{
		HasNextPage: hasNext,
		HasPrevPage: qo.Page > 1,
		Page:        qo.Page,
		PageSize:    pageSize,
		Total:       -1, // skipped by default
	}

	if !qo.SkipCount {
		count, err := r.Count(ctx, f)
		if err == nil {
			info.Total = count
		}
	}

	return records, info, nil
}

// Exists reports whether any record matches f.
func (r *Repository) Exists(ctx context.Context, f *filter.Filter) (bool, error) {
	conn := connFromContext(ctx, r.pool)
	where, err := sqlbuild.Build(f, 0)
	if err != nil {
		return false, fmt.Errorf("%s.Exists: %w", r.schema.TableName, err)
	}
	sql := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM "%s"`, r.schema.TableName)
	if where.Clause != "" {
		sql += ` WHERE ` + where.Clause
	}
	sql += `)`
	var exists bool
	err = conn.db().QueryRow(ctx, sql, where.Args...).Scan(&exists)
	return exists, dberr.Parse(err, r.schema.TableName+".Exists")
}

// Count returns the number of matching rows.
func (r *Repository) Count(ctx context.Context, f *filter.Filter) (int64, error) {
	conn := connFromContext(ctx, r.pool)
	where, err := sqlbuild.Build(f, 0)
	if err != nil {
		return 0, fmt.Errorf("%s.Count: %w", r.schema.TableName, err)
	}
	sql := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, r.schema.TableName)
	if where.Clause != "" {
		sql += ` WHERE ` + where.Clause
	}
	var count int64
	err = conn.db().QueryRow(ctx, sql, where.Args...).Scan(&count)
	return count, dberr.Parse(err, r.schema.TableName+".Count")
}

// Aggregate runs an aggregation query against matching rows.
func (r *Repository) Aggregate(ctx context.Context, f *filter.Filter, spec driver.AggregateSpec) (driver.AggregateResult, error) {
	conn := connFromContext(ctx, r.pool)
	where, err := sqlbuild.Build(f, 0)
	if err != nil {
		return driver.AggregateResult{}, fmt.Errorf("%s.Aggregate: %w", r.schema.TableName, err)
	}

	exprs := make([]string, len(spec.Functions))
	for i, fn := range spec.Functions {
		alias := fn.Alias
		if alias == "" {
			alias = fmt.Sprintf("%s_%s", fn.Fn, fn.Field)
		}
		if fn.Fn == driver.AggregateFnCount {
			exprs[i] = fmt.Sprintf(`COUNT(*) AS "%s"`, alias)
		} else {
			exprs[i] = fmt.Sprintf(`%s("%s") AS "%s"`, strings.ToUpper(string(fn.Fn)), fn.Field, alias)
		}
	}

	sql := fmt.Sprintf(`SELECT %s FROM "%s"`, strings.Join(exprs, ", "), r.schema.TableName)
	if where.Clause != "" {
		sql += ` WHERE ` + where.Clause
	}
	if spec.GroupBy != "" {
		sql += fmt.Sprintf(` GROUP BY "%s"`, spec.GroupBy)
	}

	rows, err := conn.db().Query(ctx, sql, where.Args...)
	if err != nil {
		return driver.AggregateResult{}, dberr.Parse(err, r.schema.TableName+".Aggregate")
	}
	defer rows.Close()

	result := driver.AggregateResult{Values: make(map[string]any)}
	if !rows.Next() {
		return result, rows.Err()
	}
	vals, err := rows.Values()
	if err != nil {
		return result, dberr.Parse(err, r.schema.TableName+".Aggregate.scan")
	}
	for i, fn := range spec.Functions {
		alias := fn.Alias
		if alias == "" {
			alias = fmt.Sprintf("%s_%s", fn.Fn, fn.Field)
		}
		if i < len(vals) {
			result.Values[alias] = vals[i]
		}
	}
	return result, nil
}

// -------------------------------------------------------------------------
// Write operations
// -------------------------------------------------------------------------

// Create inserts a new record. TenantID is sourced from TenantContext in ctx.
func (r *Repository) Create(ctx context.Context, input driver.CreateInput) (*def.EntityRecord, error) {
	tc := tenant.FromContext(ctx) // panics if no TenantContext — intentional
	conn := connFromContext(ctx, r.pool)
	id := uuid.New() // TODO: switch to UUIDv7 when available
	now := time.Now().UTC()

	if r.schema.IsSystem {
		return r.createSystem(ctx, conn.db(), id, tc.TenantID, input, now)
	}
	return r.createCustom(ctx, conn.db(), id, tc.TenantID, input, now)
}

func (r *Repository) createSystem(ctx context.Context, db execer, id, tenantID uuid.UUID, input driver.CreateInput, now time.Time) (*def.EntityRecord, error) {
	fieldNames := r.sortedFieldNames()
	cols := make([]string, 0, 4+len(fieldNames)+1)
	cols = append(cols, `"id"`, `"tenant_id"`, `"created_at"`, `"updated_at"`)
	args := []any{id, tenantID, now, now}

	for _, name := range fieldNames {
		cols = append(cols, fmt.Sprintf(`"%s"`, name))
		args = append(args, input.Data[name])
	}
	if len(input.CustomFields) > 0 {
		cfJSON, err := json.Marshal(input.CustomFields)
		if err != nil {
			return nil, fmt.Errorf("%s.Create: marshal custom_fields: %w", r.schema.TableName, err)
		}
		cols = append(cols, `"custom_fields"`)
		args = append(args, cfJSON)
	}

	placeholders := make([]string, len(args))
	for i := range args {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	sql := fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s)`,
		r.schema.TableName,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)
	if _, err := db.Exec(ctx, sql, args...); err != nil {
		return nil, dberr.Parse(err, r.schema.TableName+".Create")
	}
	return r.getWithDB(ctx, db, id)
}

func (r *Repository) createCustom(ctx context.Context, db execer, id, tenantID uuid.UUID, input driver.CreateInput, now time.Time) (*def.EntityRecord, error) {
	dataJSON, err := json.Marshal(input.Data)
	if err != nil {
		return nil, fmt.Errorf("%s.Create: marshal data: %w", r.schema.TableName, err)
	}
	sql := fmt.Sprintf(
		`INSERT INTO "%s" (id, tenant_id, data, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		r.schema.TableName,
	)
	if _, err := db.Exec(ctx, sql, id, tenantID, dataJSON, now, now); err != nil {
		return nil, dberr.Parse(err, r.schema.TableName+".Create")
	}
	return r.getWithDB(ctx, db, id)
}

// Update applies a partial patch to an existing record.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, input driver.UpdateInput) (*def.EntityRecord, error) {
	conn := connFromContext(ctx, r.pool)
	now := time.Now().UTC()

	if r.schema.IsSystem {
		return r.updateSystem(ctx, conn.db(), id, input, now)
	}
	return r.updateCustom(ctx, conn.db(), id, input, now)
}

func (r *Repository) updateSystem(ctx context.Context, db execer, id uuid.UUID, input driver.UpdateInput, now time.Time) (*def.EntityRecord, error) {
	if len(input.Data) == 0 && len(input.CustomFields) == 0 {
		return r.getWithDB(ctx, db, id)
	}
	sets := []string{`"updated_at" = $1`}
	args := []any{now}
	for field, val := range input.Data {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf(`"%s" = $%d`, field, len(args)))
	}
	if len(input.CustomFields) > 0 {
		cfJSON, err := json.Marshal(input.CustomFields)
		if err != nil {
			return nil, fmt.Errorf("%s.Update: marshal custom_fields: %w", r.schema.TableName, err)
		}
		args = append(args, cfJSON)
		sets = append(sets, fmt.Sprintf(`"custom_fields" = "custom_fields" || $%d`, len(args)))
	}
	args = append(args, id)
	sql := fmt.Sprintf(`UPDATE "%s" SET %s WHERE "id" = $%d`, r.schema.TableName, strings.Join(sets, ", "), len(args))
	tag, err := db.Exec(ctx, sql, args...)
	if err != nil {
		return nil, dberr.Parse(err, r.schema.TableName+".Update")
	}
	if tag.RowsAffected() == 0 {
		return nil, &runtime.NotFoundError{EntityName: r.schema.TableName, ID: id.String()}
	}
	return r.getWithDB(ctx, db, id)
}

func (r *Repository) updateCustom(ctx context.Context, db execer, id uuid.UUID, input driver.UpdateInput, now time.Time) (*def.EntityRecord, error) {
	if len(input.Data) == 0 {
		return r.getWithDB(ctx, db, id)
	}
	patchJSON, err := json.Marshal(input.Data)
	if err != nil {
		return nil, fmt.Errorf("%s.Update: marshal patch: %w", r.schema.TableName, err)
	}
	sql := fmt.Sprintf(`UPDATE "%s" SET "data" = "data" || $1, "updated_at" = $2 WHERE "id" = $3`, r.schema.TableName)
	tag, err := db.Exec(ctx, sql, patchJSON, now, id)
	if err != nil {
		return nil, dberr.Parse(err, r.schema.TableName+".Update")
	}
	if tag.RowsAffected() == 0 {
		return nil, &runtime.NotFoundError{EntityName: r.schema.TableName, ID: id.String()}
	}
	return r.getWithDB(ctx, db, id)
}

// Delete removes a record by ID.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	conn := connFromContext(ctx, r.pool)
	sql := fmt.Sprintf(`DELETE FROM "%s" WHERE "id" = $1`, r.schema.TableName)
	tag, err := conn.db().Exec(ctx, sql, id)
	if err != nil {
		return dberr.Parse(err, r.schema.TableName+".Delete")
	}
	if tag.RowsAffected() == 0 {
		return &runtime.NotFoundError{EntityName: r.schema.TableName, ID: id.String()}
	}
	return nil
}

// BulkCreate inserts multiple records atomically using a single pgx.Batch round
// trip followed by one SELECT to fetch all inserted rows.
//
// All records are inserted in the same transaction. If any insert fails the
// entire batch is rolled back. Hooks do not run — BulkCreate is a direct
// repository operation for bulk-load and import scenarios.
//
// The empty-slice case returns (nil, nil) without touching the database.
func (r *Repository) BulkCreate(ctx context.Context, inputs []driver.CreateInput) ([]*def.EntityRecord, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	tc := tenant.FromContext(ctx)
	var results []*def.EntityRecord
	err := r.WithTx(ctx, func(txCtx context.Context) error {
		conn := connFromContext(txCtx, r.pool)
		db := conn.db()
		now := time.Now().UTC()
		var err error
		if r.schema.IsSystem {
			results, err = r.bulkCreateSystem(txCtx, db, tc.TenantID, inputs, now)
		} else {
			results, err = r.bulkCreateCustom(txCtx, db, tc.TenantID, inputs, now)
		}
		return err
	})
	return results, err
}

// bulkCreateSystem sends all system-entity INSERTs in one pgx.Batch, then
// fetches inserted rows with a single WHERE id = ANY($1) query.
func (r *Repository) bulkCreateSystem(ctx context.Context, db execer, tenantID uuid.UUID, inputs []driver.CreateInput, now time.Time) ([]*def.EntityRecord, error) {
	fieldNames := r.sortedFieldNames()

	// Determine whether any input carries custom_fields.
	hasCustomFields := false
	for _, inp := range inputs {
		if len(inp.CustomFields) > 0 {
			hasCustomFields = true
			break
		}
	}

	// Column list is constant across all rows.
	cols := make([]string, 0, 4+len(fieldNames)+1)
	cols = append(cols, `"id"`, `"tenant_id"`, `"created_at"`, `"updated_at"`)
	for _, name := range fieldNames {
		cols = append(cols, fmt.Sprintf(`"%s"`, name))
	}
	if hasCustomFields {
		cols = append(cols, `"custom_fields"`)
	}
	colList := strings.Join(cols, ", ")

	batch := &pgxlib.Batch{}
	ids := make([]uuid.UUID, len(inputs))

	for i, input := range inputs {
		id := uuid.New()
		ids[i] = id

		args := make([]any, 0, 4+len(fieldNames)+1)
		args = append(args, id, tenantID, now, now)
		for _, name := range fieldNames {
			args = append(args, input.Data[name])
		}
		if hasCustomFields {
			cfJSON, err := json.Marshal(input.CustomFields)
			if err != nil {
				return nil, fmt.Errorf("%s.BulkCreate[%d]: marshal custom_fields: %w", r.schema.TableName, i, err)
			}
			args = append(args, cfJSON)
		}

		placeholders := make([]string, len(args))
		for j := range args {
			placeholders[j] = fmt.Sprintf("$%d", j+1)
		}
		sql := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
			r.schema.TableName, colList, strings.Join(placeholders, ", "))
		batch.Queue(sql, args...)
	}

	if err := r.sendBatch(ctx, db, batch, len(inputs), r.schema.TableName+".BulkCreate"); err != nil {
		return nil, err
	}
	return r.getByIDs(ctx, db, ids)
}

// bulkCreateCustom sends all custom-entity INSERTs in one pgx.Batch.
func (r *Repository) bulkCreateCustom(ctx context.Context, db execer, tenantID uuid.UUID, inputs []driver.CreateInput, now time.Time) ([]*def.EntityRecord, error) {
	insertSQL := fmt.Sprintf(
		`INSERT INTO "%s" (id, tenant_id, data, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		r.schema.TableName,
	)

	batch := &pgxlib.Batch{}
	ids := make([]uuid.UUID, len(inputs))

	for i, input := range inputs {
		id := uuid.New()
		ids[i] = id
		dataJSON, err := json.Marshal(input.Data)
		if err != nil {
			return nil, fmt.Errorf("%s.BulkCreate[%d]: marshal data: %w", r.schema.TableName, i, err)
		}
		batch.Queue(insertSQL, id, tenantID, dataJSON, now, now)
	}

	if err := r.sendBatch(ctx, db, batch, len(inputs), r.schema.TableName+".BulkCreate"); err != nil {
		return nil, err
	}
	return r.getByIDs(ctx, db, ids)
}

// sendBatch sends b and drains n Exec results. Any error stops draining and
// closes the BatchResults before returning.
func (r *Repository) sendBatch(ctx context.Context, db execer, b *pgxlib.Batch, n int, op string) error {
	br := db.SendBatch(ctx, b)
	for i := range n {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return dberr.Parse(err, fmt.Sprintf("%s[%d]", op, i))
		}
	}
	return br.Close()
}

// getByIDs fetches all records whose id is in the ids slice.
// Order matches insertion order (ORDER BY created_at, id for stability).
func (r *Repository) getByIDs(ctx context.Context, db execer, ids []uuid.UUID) ([]*def.EntityRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cols, scan := r.columnsAndScanner()
	sql := fmt.Sprintf(
		`SELECT %s FROM "%s" WHERE "id" = ANY($1) ORDER BY "created_at", "id"`,
		cols, r.schema.TableName,
	)
	rows, err := db.Query(ctx, sql, ids)
	if err != nil {
		return nil, dberr.Parse(err, r.schema.TableName+".BulkCreate.fetch")
	}
	defer rows.Close()
	results := make([]*def.EntityRecord, 0, len(ids))
	for rows.Next() {
		rec, err := scan(rows)
		if err != nil {
			return nil, dberr.Parse(err, r.schema.TableName+".BulkCreate.scan")
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

// BulkUpdate patches all records matching f. Hooks do not run.
func (r *Repository) BulkUpdate(ctx context.Context, f *filter.Filter, patch driver.Patch) (int64, error) {
	conn := connFromContext(ctx, r.pool)
	where, err := sqlbuild.Build(f, 0)
	if err != nil {
		return 0, fmt.Errorf("%s.BulkUpdate: %w", r.schema.TableName, err)
	}

	now := time.Now().UTC()

	if r.schema.IsSystem {
		sets := []string{`"updated_at" = $1`}
		args := []any{now}
		for field, val := range patch.Set {
			args = append(args, val)
			sets = append(sets, fmt.Sprintf(`"%s" = $%d`, field, len(args)))
		}
		// Shift where args after the SET args.
		whereArgs := make([]any, len(where.Args))
		copy(whereArgs, where.Args)
		whereOffset := len(args)
		shiftedClause := shiftParams(where.Clause, whereOffset)
		args = append(args, whereArgs...)

		sql := fmt.Sprintf(`UPDATE "%s" SET %s`, r.schema.TableName, strings.Join(sets, ", "))
		if shiftedClause != "" {
			sql += ` WHERE ` + shiftedClause
		}
		tag, err2 := conn.db().Exec(ctx, sql, args...)
		return tag.RowsAffected(), dberr.Parse(err2, r.schema.TableName+".BulkUpdate")
	}

	// Custom entity: merge patch into jsonb.
	patchJSON, err := json.Marshal(patch.Set)
	if err != nil {
		return 0, fmt.Errorf("%s.BulkUpdate: marshal patch: %w", r.schema.TableName, err)
	}
	// $1 = patchJSON, $2 = now; where args shift by 2.
	args := []any{patchJSON, now}
	shiftedClause := shiftParams(where.Clause, 2)
	args = append(args, where.Args...)
	sql := fmt.Sprintf(`UPDATE "%s" SET "data" = "data" || $1, "updated_at" = $2`, r.schema.TableName)
	if shiftedClause != "" {
		sql += ` WHERE ` + shiftedClause
	}
	tag, err2 := conn.db().Exec(ctx, sql, args...)
	return tag.RowsAffected(), dberr.Parse(err2, r.schema.TableName+".BulkUpdate")
}

// WithTx executes fn inside a database transaction.
func (r *Repository) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	existing := connFromContext(ctx, r.pool)
	if existing.InTx() {
		return fn(ctx)
	}

	pgxTx, err := r.pool.BeginTx(ctx, pgxlib.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	conn := &pgConn{pool: r.pool, txn: pgxTx}
	txCtx := tx.WithConn(ctx, conn)

	if tc, ok := tenant.TryFromContext(ctx); ok {
		if err := setTenantContext(txCtx, conn.db(), tc.TenantID.String()); err != nil {
			_ = pgxTx.Rollback(ctx)
			return fmt.Errorf("set_tenant_context: %w", err)
		}
	}

	if err := fn(txCtx); err != nil {
		_ = pgxTx.Rollback(ctx)
		return err
	}
	return pgxTx.Commit(ctx)
}

// -------------------------------------------------------------------------
// Internal helpers
// -------------------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *Repository) columnsAndScanner() (string, func(rowScanner) (*def.EntityRecord, error)) {
	if r.schema.IsSystem {
		return r.systemColumnsAndScanner()
	}
	return r.customColumnsAndScanner()
}

func (r *Repository) systemColumnsAndScanner() (string, func(rowScanner) (*def.EntityRecord, error)) {
	table := r.schema.TableName
	fieldNames := r.sortedFieldNames()

	colExprs := make([]string, 0, 4+len(fieldNames)+1)
	colExprs = append(colExprs,
		fmt.Sprintf(`"%s"."id"`, table),
		fmt.Sprintf(`"%s"."tenant_id"`, table),
		fmt.Sprintf(`"%s"."created_at"`, table),
		fmt.Sprintf(`"%s"."updated_at"`, table),
	)
	for _, name := range fieldNames {
		colExprs = append(colExprs, fmt.Sprintf(`"%s"."%s"`, table, name))
	}
	colExprs = append(colExprs, fmt.Sprintf(`"%s"."custom_fields"`, table))

	scan := func(row rowScanner) (*def.EntityRecord, error) {
		rec := &def.EntityRecord{
			EntityName: table,
			Data:       make(map[string]any, len(fieldNames)),
		}
		dest := make([]any, 0, 4+len(fieldNames)+1)
		dest = append(dest, &rec.ID, &rec.TenantID, &rec.CreatedAt, &rec.UpdatedAt)

		fieldPtrs := make([]any, len(fieldNames))
		for i := range fieldNames {
			var v any
			fieldPtrs[i] = &v
			dest = append(dest, fieldPtrs[i])
		}
		var cfRaw []byte
		dest = append(dest, &cfRaw)

		if err := row.Scan(dest...); err != nil {
			return nil, err
		}
		for i, name := range fieldNames {
			rec.Data[name] = *(fieldPtrs[i].(*any))
		}
		if len(cfRaw) > 0 {
			_ = json.Unmarshal(cfRaw, &rec.CustomFields)
		}
		return rec, nil
	}
	return strings.Join(colExprs, ", "), scan
}

func (r *Repository) customColumnsAndScanner() (string, func(rowScanner) (*def.EntityRecord, error)) {
	table := r.schema.TableName
	cols := fmt.Sprintf(
		`"%s"."id", "%s"."tenant_id", "%s"."data", "%s"."created_at", "%s"."updated_at"`,
		table, table, table, table, table,
	)
	scan := func(row rowScanner) (*def.EntityRecord, error) {
		rec := &def.EntityRecord{EntityName: table}
		var dataRaw []byte
		if err := row.Scan(&rec.ID, &rec.TenantID, &dataRaw, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		if len(dataRaw) > 0 {
			_ = json.Unmarshal(dataRaw, &rec.Data)
		}
		if rec.Data == nil {
			rec.Data = make(map[string]any)
		}
		return rec, nil
	}
	return cols, scan
}

// sortedFieldNames returns field names in a deterministic order (alphabetical).
// Consistent column order is required for INSERT/SELECT to match correctly.
func (r *Repository) sortedFieldNames() []string {
	names := make([]string, 0, len(r.schema.FieldsByName))
	for name := range r.schema.FieldsByName {
		names = append(names, name)
	}
	// Simple insertion sort — field count is tiny (< 100).
	for i := 1; i < len(names); i++ {
		key := names[i]
		j := i - 1
		for j >= 0 && names[j] > key {
			names[j+1] = names[j]
			j--
		}
		names[j+1] = key
	}
	return names
}

// getWithDB fetches a record using a specific execer (useful inside transactions).
func (r *Repository) getWithDB(ctx context.Context, db execer, id uuid.UUID) (*def.EntityRecord, error) {
	cols, scan := r.columnsAndScanner()
	sql := fmt.Sprintf(`SELECT %s FROM "%s" WHERE "id" = $1 LIMIT 1`, cols, r.schema.TableName)
	rec, err := scan(db.QueryRow(ctx, sql, id))
	if err != nil {
		if err == pgxlib.ErrNoRows {
			return nil, &runtime.NotFoundError{EntityName: r.schema.TableName, ID: id.String()}
		}
		return nil, dberr.Parse(err, r.schema.TableName+".get")
	}
	return rec, nil
}

// shiftParams rewrites $1,$2,... in clause to $1+offset,$2+offset,...
// Used when prepending SET arguments before WHERE arguments.
func shiftParams(clause string, offset int) string {
	if clause == "" || offset == 0 {
		return clause
	}
	// Walk backwards through large-numbered params first to avoid double-replace.
	// Simple approach: use strings.Builder with character scanning.
	var sb strings.Builder
	i := 0
	for i < len(clause) {
		if clause[i] == '$' && i+1 < len(clause) {
			j := i + 1
			for j < len(clause) && clause[j] >= '0' && clause[j] <= '9' {
				j++
			}
			if j > i+1 {
				var n int
				fmt.Sscanf(clause[i+1:j], "%d", &n)
				sb.WriteString(fmt.Sprintf("$%d", n+offset))
				i = j
				continue
			}
		}
		sb.WriteByte(clause[i])
		i++
	}
	return sb.String()
}

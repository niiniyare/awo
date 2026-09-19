package ioport_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/registry"

	. "awo.so/awo/ioport"
)

// --- helpers ---

func buildEntitySchema(t *testing.T, d def.EntityDefinition) *compiler.EntitySchema {
	t.Helper()
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	es, ok := schema.ByName[d.EntityName()]
	if !ok {
		t.Fatalf("entity %q not found in schema", d.EntityName())
	}
	return es
}

func productDef() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:        "io_product",
		Module:      "io",
		Label:       "Product",
		LabelPlural: "Products",
		Fields: []def.FieldDef{
			{Name: "sku", Type: def.FieldTypeData, Required: true},
			{Name: "price", Type: def.FieldTypeCurrency},
			{Name: "quantity", Type: def.FieldTypeInt},
			{Name: "active", Type: def.FieldTypeBool},
			{Name: "secret", Type: def.FieldTypeData, Sensitive: true},
		},
		Permissions: def.PermissionSet{Read: []string{"io.product.read"}},
	}
}

// mockRepo is a minimal in-memory EntityRepository for Export tests, which
// remain on the plain read-only driver.EntityRepository path (Export
// performs no mutation and needs no pipeline access — Phase 2 Step 5 only
// changes Import's persistence dependency).
type mockRepo struct {
	records []*def.EntityRecord
}

func (m *mockRepo) Query(_ context.Context, _ *filter.Filter, _ ...driver.QueryOption) ([]*def.EntityRecord, driver.PageInfo, error) {
	return m.records, driver.PageInfo{Total: int64(len(m.records))}, nil
}

// Stub implementations for the remaining interface methods (unused by Export).
func (m *mockRepo) Get(_ context.Context, _ uuid.UUID, _ ...driver.QueryOption) (*def.EntityRecord, error) {
	return nil, errors.New("not implemented")
}
func (m *mockRepo) Exists(_ context.Context, _ *filter.Filter) (bool, error) { return false, nil }
func (m *mockRepo) Count(_ context.Context, _ *filter.Filter) (int64, error) { return 0, nil }
func (m *mockRepo) Aggregate(_ context.Context, _ *filter.Filter, _ driver.AggregateSpec) (driver.AggregateResult, error) {
	return driver.AggregateResult{}, nil
}
func (m *mockRepo) Create(_ context.Context, _ driver.CreateInput) (*def.EntityRecord, error) {
	return nil, errors.New("not implemented")
}
func (m *mockRepo) Update(_ context.Context, _ uuid.UUID, _ driver.UpdateInput) (*def.EntityRecord, error) {
	return nil, errors.New("not implemented")
}
func (m *mockRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockRepo) BulkCreate(_ context.Context, _ []driver.CreateInput) ([]*def.EntityRecord, error) {
	return nil, errors.New("not implemented")
}
func (m *mockRepo) BulkUpdate(_ context.Context, _ *filter.Filter, _ driver.Patch) (int64, error) {
	return 0, nil
}
func (m *mockRepo) WithTx(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}

// fakeBatchCall records one CreateBatch invocation for assertions on how
// ioport.Import drives the EntityBatchMutator interface (flush size, actor
// propagation, SkipErrors → skipInvalid mapping).
type fakeBatchCall struct {
	rows        []map[string]any
	actor       *def.Actor
	skipInvalid bool
}

// fakeMutator implements ioport.EntityBatchMutator for testing ioport's own
// import control flow (batch assembly, row-number attribution, SkipErrors
// handling) fast and without a database. It does not perform real
// validation/hooks/audit/outbox — that is EntityService.CreateBatch's own
// implementation, exercised against real PostgreSQL in
// api/service/entity_batch_pg_test.go. fakeMutator instead simulates
// EntityBatchMutator's documented contract closely enough to prove ioport
// interprets it correctly: per-row validation failures (invalidAt) become
// Skipped entries (or abort the call, mirroring skipInvalid), and a
// transactional failure (bulkErr) fails the whole flush.
type fakeMutator struct {
	records []*def.EntityRecord

	// bulkErr, when non-nil, is returned as the flush-transaction failure
	// from every CreateBatch call whose valid-row set is non-empty.
	bulkErr error

	// invalidAt marks 0-based row indices, relative to each individual
	// CreateBatch call (not the whole import), that should be reported as
	// RunBeforeCreate-style validation failures.
	invalidAt map[int]error

	calls []fakeBatchCall
}

func (m *fakeMutator) CreateBatch(_ context.Context, rows []map[string]any, actor *def.Actor, skipInvalid bool) (*def.BatchCreateResult, error) {
	m.calls = append(m.calls, fakeBatchCall{rows: rows, actor: actor, skipInvalid: skipInvalid})

	var valid []map[string]any
	var skipped []def.BatchRowError
	for i, row := range rows {
		if err, bad := m.invalidAt[i]; bad {
			if !skipInvalid {
				return nil, err
			}
			skipped = append(skipped, def.BatchRowError{Index: i, Err: err})
			continue
		}
		valid = append(valid, row)
	}

	if len(valid) == 0 {
		return &def.BatchCreateResult{Skipped: skipped}, nil
	}
	if m.bulkErr != nil {
		return &def.BatchCreateResult{Skipped: skipped}, m.bulkErr
	}

	created := make([]*def.EntityRecord, len(valid))
	for i, data := range valid {
		created[i] = &def.EntityRecord{ID: uuid.New(), Data: data}
		m.records = append(m.records, created[i])
	}
	return &def.BatchCreateResult{Created: created, Skipped: skipped}, nil
}

// --- Import tests ---

func TestImport_CSV_CreatesRecords(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{}

	csv := "sku,price\nSKU-001,9.99\nSKU-002,19.99\n"
	result, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{
		Format: FormatCSV,
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected Total=2, got %d", result.Total)
	}
	if result.Created != 2 {
		t.Errorf("expected Created=2, got %d", result.Created)
	}
	if len(mutator.records) != 2 {
		t.Errorf("expected 2 records in mutator, got %d", len(mutator.records))
	}
}

func TestImport_CSV_UnknownColumnsIgnored(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{}

	csv := "sku,nonexistent_field\nSKU-001,blah\n"
	result, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected Total=1, got %d", result.Total)
	}
	// Record should exist with just sku.
	if len(mutator.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(mutator.records))
	}
	if mutator.records[0].Data["sku"] != "SKU-001" {
		t.Errorf("expected sku=SKU-001, got %v", mutator.records[0].Data["sku"])
	}
}

func TestImport_CSV_SkipErrors(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{bulkErr: errors.New("db error")}

	// BatchSize=1 forces a flush inside the row loop, which respects SkipErrors.
	// The final flush is a no-op (empty batch) so no unconditional error path.
	csv := "sku\nSKU-001\nSKU-002\n"
	result, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{
		SkipErrors: true,
		BatchSize:  1,
	})
	if err != nil {
		t.Fatalf("Import with SkipErrors should not return error, got: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one row error recorded")
	}
	if result.Created != 0 {
		t.Errorf("expected Created=0 (every flush failed), got %d", result.Created)
	}
}

func TestImport_CSV_TransactionalFailure_AbortsWholeImport_WhenSkipErrorsFalse(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{bulkErr: errors.New("db error")}

	csv := "sku\nSKU-001\nSKU-002\n"
	result, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{
		SkipErrors: false,
		BatchSize:  100,
	})
	if err == nil {
		t.Fatal("expected Import to fail when the flush transaction fails and SkipErrors is false")
	}
	if result.Created != 0 {
		t.Errorf("no record may be reported as created when its flush transaction failed, got Created=%d", result.Created)
	}
}

func TestImport_CSV_ValidationFailure_ExcludedRow_RestOfFlushProceeds_WhenSkipErrorsTrue(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	// Row index 1 (0-based, within the single flush) fails validation;
	// rows 0 and 2 are valid — matching ADR-025 §14: with SkipErrors: true,
	// the invalid row is excluded but the rest of the flush still commits.
	mutator := &fakeMutator{invalidAt: map[int]error{1: errors.New("sku is required")}}

	// All 3 rows land in one flush (BatchSize=100); fakeMutator marks the
	// row at 0-based index 1 within that flush (SKU-002, input row 3 —
	// header is row 0, rowNum increments before each data row is read) as
	// failing validation.
	csv := "sku\nSKU-001\nSKU-002\nSKU-003\n"
	result, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{
		SkipErrors: true,
		BatchSize:  100,
	})
	if err != nil {
		t.Fatalf("Import with SkipErrors should not return error, got: %v", err)
	}
	if result.Created != 2 {
		t.Errorf("expected the 2 valid rows to be created despite 1 invalid row, got Created=%d", result.Created)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected exactly 1 row error, got %d: %v", len(result.Errors), result.Errors)
	}
	if result.Errors[0].Row != 3 {
		t.Errorf("expected the invalid row to be attributed to input row 3 (SKU-002), got row %d", result.Errors[0].Row)
	}
}

func TestImport_CSV_ValidationFailure_AbortsWholeImport_WhenSkipErrorsFalse(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{invalidAt: map[int]error{0: errors.New("sku is required")}}

	csv := "sku\nSKU-001\nSKU-002\n"
	result, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{
		SkipErrors: false,
		BatchSize:  100,
	})
	if err == nil {
		t.Fatal("expected Import to abort when a row fails validation and SkipErrors is false")
	}
	if result.Created != 0 {
		t.Errorf("no record may be created when the flush was never opened, got Created=%d", result.Created)
	}
	if len(mutator.records) != 0 {
		t.Errorf("mutator must never have persisted anything, got %d records", len(mutator.records))
	}
}

func TestImport_JSON_CreatesRecords(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{}

	jsonInput := `[{"sku":"SKU-A","quantity":10},{"sku":"SKU-B","quantity":5}]`
	result, err := Import(context.Background(), es, mutator, strings.NewReader(jsonInput), ImportOptions{
		Format: FormatJSON,
	})
	if err != nil {
		t.Fatalf("Import JSON: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected Total=2, got %d", result.Total)
	}
	if result.Created != 2 {
		t.Errorf("expected Created=2, got %d", result.Created)
	}
}

func TestImport_JSONL_CreatesRecords(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{}

	jsonl := `{"sku":"SKU-X"}` + "\n" + `{"sku":"SKU-Y"}` + "\n"
	result, err := Import(context.Background(), es, mutator, strings.NewReader(jsonl), ImportOptions{
		Format: FormatJSONL,
	})
	if err != nil {
		t.Fatalf("Import JSONL: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected Total=2, got %d", result.Total)
	}
}

func TestImport_UnknownFormat_Error(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{}
	_, err := Import(context.Background(), es, mutator, strings.NewReader(""), ImportOptions{
		Format: "xml",
	})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestImport_CSV_ActorPropagatedToMutator(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{}
	actor := &def.Actor{UserID: uuid.New()}

	csv := "sku\nSKU-001\n"
	_, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{Actor: actor})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(mutator.calls) != 1 {
		t.Fatalf("expected 1 CreateBatch call, got %d", len(mutator.calls))
	}
	if mutator.calls[0].actor != actor {
		t.Error("expected ImportOptions.Actor to be passed through to CreateBatch verbatim")
	}
}

func TestImport_CSV_SkipInvalidMirrorsSkipErrorsOption(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	mutator := &fakeMutator{}

	csv := "sku\nSKU-001\n"
	_, err := Import(context.Background(), es, mutator, strings.NewReader(csv), ImportOptions{SkipErrors: true})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(mutator.calls) != 1 || !mutator.calls[0].skipInvalid {
		t.Error("expected ImportOptions.SkipErrors=true to be passed through to CreateBatch as skipInvalid=true")
	}
}

// --- Export tests ---

func TestExport_CSV_WritesHeaderAndRows(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{
		records: []*def.EntityRecord{
			{ID: uuid.New(), Data: map[string]any{"sku": "SKU-001", "price": "9.99"}},
		},
	}

	var buf bytes.Buffer
	err := Export(context.Background(), es, repo, &buf, ExportOptions{Format: FormatCSV})
	if err != nil {
		t.Fatalf("Export CSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "sku") {
		t.Errorf("expected 'sku' column in CSV header, got: %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("sensitive field 'secret' must not appear in export")
	}
}

func TestExport_JSON_WritesArray(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{
		records: []*def.EntityRecord{
			{ID: uuid.New(), Data: map[string]any{"sku": "SKU-001"}},
		},
	}

	var buf bytes.Buffer
	err := Export(context.Background(), es, repo, &buf, ExportOptions{Format: FormatJSON})
	if err != nil {
		t.Fatalf("Export JSON: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "SKU-001") {
		t.Errorf("expected SKU-001 in JSON export, got: %s", out)
	}
}

func TestExport_UnknownFormat_Error(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{}
	err := Export(context.Background(), es, repo, &bytes.Buffer{}, ExportOptions{Format: "toml"})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestExport_SensitiveField_Excluded(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{
		records: []*def.EntityRecord{
			{ID: uuid.New(), Data: map[string]any{"sku": "A", "secret": "TOP-SECRET"}},
		},
	}

	var buf bytes.Buffer
	_ = Export(context.Background(), es, repo, &buf, ExportOptions{Format: FormatCSV})
	if strings.Contains(buf.String(), "secret") {
		t.Error("sensitive field 'secret' must not appear in export output")
	}
}

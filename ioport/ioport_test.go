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

// mockRepo is a minimal in-memory EntityRepository for testing.
type mockRepo struct {
	records []*def.EntityRecord
	bulkErr error
}

func (m *mockRepo) BulkCreate(_ context.Context, inputs []driver.CreateInput) ([]*def.EntityRecord, error) {
	if m.bulkErr != nil {
		return nil, m.bulkErr
	}
	created := make([]*def.EntityRecord, len(inputs))
	for i, inp := range inputs {
		created[i] = &def.EntityRecord{ID: uuid.New(), Data: inp.Data}
		m.records = append(m.records, created[i])
	}
	return created, nil
}

func (m *mockRepo) Query(_ context.Context, _ *filter.Filter, _ ...driver.QueryOption) ([]*def.EntityRecord, driver.PageInfo, error) {
	return m.records, driver.PageInfo{Total: int64(len(m.records))}, nil
}

// Stub implementations for the remaining interface methods.
func (m *mockRepo) Get(_ context.Context, _ uuid.UUID, _ ...driver.QueryOption) (*def.EntityRecord, error) {
	return nil, errors.New("not implemented")
}
func (m *mockRepo) Exists(_ context.Context, _ *filter.Filter) (bool, error) { return false, nil }
func (m *mockRepo) Count(_ context.Context, _ *filter.Filter) (int64, error)  { return 0, nil }
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
func (m *mockRepo) BulkUpdate(_ context.Context, _ *filter.Filter, _ driver.Patch) (int64, error) {
	return 0, nil
}
func (m *mockRepo) WithTx(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}

// --- Import tests ---

func TestImport_CSV_CreatesRecords(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{}

	csv := "sku,price\nSKU-001,9.99\nSKU-002,19.99\n"
	result, err := Import(context.Background(), es, repo, strings.NewReader(csv), ImportOptions{
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
	if len(repo.records) != 2 {
		t.Errorf("expected 2 records in repo, got %d", len(repo.records))
	}
}

func TestImport_CSV_UnknownColumnsIgnored(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{}

	csv := "sku,nonexistent_field\nSKU-001,blah\n"
	result, err := Import(context.Background(), es, repo, strings.NewReader(csv), ImportOptions{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected Total=1, got %d", result.Total)
	}
	// Record should exist with just sku.
	if len(repo.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(repo.records))
	}
	if repo.records[0].Data["sku"] != "SKU-001" {
		t.Errorf("expected sku=SKU-001, got %v", repo.records[0].Data["sku"])
	}
}

func TestImport_CSV_SkipErrors(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{bulkErr: errors.New("db error")}

	// BatchSize=1 forces a flush inside the row loop, which respects SkipErrors.
	// The final flush is a no-op (empty batch) so no unconditional error path.
	csv := "sku\nSKU-001\nSKU-002\n"
	result, err := Import(context.Background(), es, repo, strings.NewReader(csv), ImportOptions{
		SkipErrors: true,
		BatchSize:  1,
	})
	if err != nil {
		t.Fatalf("Import with SkipErrors should not return error, got: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one row error recorded")
	}
}

func TestImport_JSON_CreatesRecords(t *testing.T) {
	es := buildEntitySchema(t, productDef())
	repo := &mockRepo{}

	jsonInput := `[{"sku":"SKU-A","quantity":10},{"sku":"SKU-B","quantity":5}]`
	result, err := Import(context.Background(), es, repo, strings.NewReader(jsonInput), ImportOptions{
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
	repo := &mockRepo{}

	jsonl := `{"sku":"SKU-X"}` + "\n" + `{"sku":"SKU-Y"}` + "\n"
	result, err := Import(context.Background(), es, repo, strings.NewReader(jsonl), ImportOptions{
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
	repo := &mockRepo{}
	_, err := Import(context.Background(), es, repo, strings.NewReader(""), ImportOptions{
		Format: "xml",
	})
	if err == nil {
		t.Fatal("expected error for unknown format")
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

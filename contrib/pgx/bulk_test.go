package pgx

import (
	"context"
	"testing"
	"time"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
)

// ── test helpers ─────────────────────────────────────────────────────────────

func testSystemSchema() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "test_thing",
		TableName:     "test_thing",
		IsSystem:      true,
		FieldsByName: map[string]def.FieldDef{
			"name": {Name: "name", Type: def.FieldTypeData},
		},
	}
}

func testCustomSchema() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "test_custom",
		TableName:     "custom_entity_records",
		IsSystem:      false,
		FieldsByName:  map[string]def.FieldDef{},
	}
}

// ── unit tests (no DB required) ───────────────────────────────────────────────

// TestBulkCreate_NilInputs verifies the empty fast path: nil slice returns
// (nil, nil) without any database interaction.
func TestBulkCreate_NilInputs(t *testing.T) {
	r := &Repository{schema: testSystemSchema()}
	results, err := r.BulkCreate(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results for nil input, got %v", results)
	}
}

// TestBulkCreate_EmptySlice verifies the empty fast path: empty slice returns
// (nil, nil) without any database interaction.
func TestBulkCreate_EmptySlice(t *testing.T) {
	r := &Repository{schema: testSystemSchema()}
	results, err := r.BulkCreate(context.Background(), []driver.CreateInput{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results for empty input, got %d records", len(results))
	}
}

// TestGetByIDs_Empty verifies getByIDs short-circuits on nil/empty id list.
func TestGetByIDs_Empty(t *testing.T) {
	r := &Repository{schema: testSystemSchema()}
	results, err := r.getByIDs(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results for empty ids, got %v", results)
	}
}

// TestBulkCreateCustom_MarshalError verifies that a non-serialisable value in
// input Data is detected and returned as an error before any batch is sent.
// Channels cannot be marshalled to JSON.
func TestBulkCreateCustom_MarshalError(t *testing.T) {
	r := &Repository{schema: testCustomSchema()}
	inputs := []driver.CreateInput{
		{Data: map[string]any{"ok": "fine"}},
		{Data: map[string]any{"bad": make(chan int)}}, // channel → json.Marshal error
	}
	_, err := r.bulkCreateCustom(
		context.Background(),
		nil,          // db — not reached because marshal fails first
		[16]byte{},   // tenantID
		inputs,
		time.Now().UTC(),
	)
	if err == nil {
		t.Fatal("expected marshal error for channel value, got nil")
	}
}

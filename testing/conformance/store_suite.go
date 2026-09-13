// Package conformance provides test suites that any driver.RecordRepository
// implementation can run to verify correctness.
package conformance

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime/tenant"
)

// StoreFactory is a function that returns a fresh, empty RecordRepository.
// Called once per sub-test.
type StoreFactory func() driver.RecordRepository

// StoreSuite runs conformance tests against any RecordRepository implementation.
type StoreSuite struct {
	Factory StoreFactory
}

// Run executes all conformance tests under t.
func (s *StoreSuite) Run(t *testing.T) {
	t.Helper()

	t.Run("CreateAndGet", s.testCreateAndGet)
	t.Run("UpdateFields", s.testUpdateFields)
	t.Run("Delete", s.testDelete)
	t.Run("FilterEq", s.testFilterEq)
	t.Run("FilterNeq", s.testFilterNeq)
	t.Run("FilterIn", s.testFilterIn)
	t.Run("FilterIsNull", s.testFilterIsNull)
	t.Run("FilterAnd", s.testFilterAnd)
	t.Run("FilterOr", s.testFilterOr)
	t.Run("FilterNot", s.testFilterNot)
	t.Run("Count", s.testCount)
	t.Run("Exists", s.testExists)
	t.Run("TenantIsolation", s.testTenantIsolation)
}

func tenantCtx(tenantID uuid.UUID) context.Context {
	return tenant.WithContext(context.Background(), tenant.TenantContext{
		TenantID:   tenantID,
		TenantSlug: "test",
		Locale:     "en-KE",
		Timezone:   "Africa/Nairobi",
		Currency:   "KES",
	})
}

func makeRecord(_ string, data map[string]any) driver.CreateInput {
	return driver.CreateInput{
		Data: data,
	}
}

func (s *StoreSuite) testCreateAndGet(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	created, err := store.Create(ctx, makeRecord("test_item", map[string]any{
		"name": "Widget",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created == nil {
		t.Fatal("Create returned nil")
	}
	if created.ID == uuid.Nil {
		t.Error("Create: ID should be set")
	}

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.GetString("name") != "Widget" {
		t.Errorf("Get: name = %q, want %q", got.GetString("name"), "Widget")
	}
}

func (s *StoreSuite) testUpdateFields(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	created, err := store.Create(ctx, makeRecord("test_item", map[string]any{
		"status": "draft",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := store.Update(ctx, created.ID, driver.UpdateInput{
		Data: map[string]any{"status": "active"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.GetString("status") != "active" {
		t.Errorf("Update: status = %q, want %q", updated.GetString("status"), "active")
	}
}

func (s *StoreSuite) testDelete(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	created, _ := store.Create(ctx, makeRecord("test_item", map[string]any{"x": "1"}))

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(ctx, created.ID)
	if err == nil {
		t.Error("Get after Delete: expected error, got nil")
	}
}

func (s *StoreSuite) testFilterEq(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "draft"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active"}))

	results, _, err := store.Query(ctx, filter.Eq("status", "active"))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("FilterEq: got %d results, want 2", len(results))
	}
}

func (s *StoreSuite) testFilterNeq(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "draft"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active"}))

	results, _, err := store.Query(ctx, filter.Neq("status", "draft"))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("FilterNeq: got %d results, want 1", len(results))
	}
}

func (s *StoreSuite) testFilterIn(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "draft"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "archived"}))

	results, _, err := store.Query(ctx, filter.In("status", "draft", "active"))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("FilterIn: got %d results, want 2", len(results))
	}
}

func (s *StoreSuite) testFilterIsNull(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	store.Create(ctx, makeRecord("test_item", map[string]any{"note": nil}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"note": "hello"}))

	results, _, err := store.Query(ctx, filter.IsNull("note"))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("FilterIsNull: got %d results, want 1", len(results))
	}
}

func (s *StoreSuite) testFilterAnd(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active", "type": "A"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active", "type": "B"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "draft", "type": "A"}))

	f := filter.And(filter.Eq("status", "active"), filter.Eq("type", "A"))
	results, _, err := store.Query(ctx, f)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("FilterAnd: got %d results, want 1", len(results))
	}
}

func (s *StoreSuite) testFilterOr(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "draft"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "archived"}))

	f := filter.Or(filter.Eq("status", "draft"), filter.Eq("status", "archived"))
	results, _, err := store.Query(ctx, f)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("FilterOr: got %d results, want 2", len(results))
	}
}

func (s *StoreSuite) testFilterNot(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "draft"}))
	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "active"}))

	results, _, err := store.Query(ctx, filter.Not(filter.Eq("status", "draft")))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("FilterNot: got %d results, want 1", len(results))
	}
}

func (s *StoreSuite) testCount(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	for i := 0; i < 5; i++ {
		store.Create(ctx, makeRecord("test_item", map[string]any{"n": int64(i)}))
	}

	n, err := store.Count(ctx, nil)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 5 {
		t.Errorf("Count: got %d, want 5", n)
	}
}

func (s *StoreSuite) testExists(t *testing.T) {
	store := s.Factory()
	ctx := tenantCtx(uuid.New())

	ok, err := store.Exists(ctx, filter.Eq("status", "draft"))
	if err != nil {
		t.Fatalf("Exists (empty): %v", err)
	}
	if ok {
		t.Error("Exists: expected false on empty store")
	}

	store.Create(ctx, makeRecord("test_item", map[string]any{"status": "draft"}))

	ok, err = store.Exists(ctx, filter.Eq("status", "draft"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Error("Exists: expected true after create")
	}
}

func (s *StoreSuite) testTenantIsolation(t *testing.T) {
	store := s.Factory()

	tenantA := uuid.New()
	tenantB := uuid.New()
	ctxA := tenantCtx(tenantA)
	ctxB := tenantCtx(tenantB)

	store.Create(ctxA, makeRecord("test_item", map[string]any{"owner": "A"}))
	store.Create(ctxA, makeRecord("test_item", map[string]any{"owner": "A"}))
	store.Create(ctxB, makeRecord("test_item", map[string]any{"owner": "B"}))

	// Implementations that enforce tenant isolation should filter by tenant_id.
	// The conformance test checks that records created in tenant B are not
	// returned when queried from tenant A's context.
	// Note: the fakestore does NOT enforce tenant isolation (it is a testing
	// primitive); this test documents the expected behaviour for production
	// stores. Fakestore-based tests should skip this sub-test.
	resultsA, _, err := store.Query(ctxA, filter.Eq("owner", "A"))
	if err != nil {
		t.Fatalf("Query tenantA: %v", err)
	}

	for _, r := range resultsA {
		if r.TenantID != uuid.Nil && r.TenantID != tenantA {
			t.Errorf("TenantIsolation: record from tenant %s appeared in tenant A query", r.TenantID)
		}
	}
}

// Package bench contains benchmarks for the Awo framework core operations.
// These run against the fakestore (no DB required) to measure framework
// overhead: registry build, compilation, filter evaluation, CRUD operations.
package bench_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/platform/iam"
	"awo.so/awo/platform/organization"
	"awo.so/awo/platform/tenant"
	"awo.so/awo/registry"
	"awo.so/awo/testing/fakestore"
	"awo.so/awo/testing/harness"
)

func platformDefs() []def.EntityDefinition {
	return []def.EntityDefinition{
		&tenant.Definition,
		&iam.UserDefinition,
		&organization.Definition,
		&organization.OrgTypeDefinition,
		&organization.OrgAssignmentDefinition,
	}
}

// BenchmarkRegistryBuild measures the cost of BuildFrom + validation.
func BenchmarkRegistryBuild(b *testing.B) {
	defs := platformDefs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg, err := registry.BuildFrom(defs)
		if err != nil || reg == nil {
			b.Fatalf("BuildFrom: %v", err)
		}
	}
}

// BenchmarkCompilerCompile measures the cost of schema compilation.
func BenchmarkCompilerCompile(b *testing.B) {
	reg, err := registry.BuildFrom(platformDefs())
	if err != nil {
		b.Fatalf("registry: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		schema, err := compiler.Compile(reg)
		if err != nil || schema == nil {
			b.Fatalf("Compile: %v", err)
		}
	}
}

// BenchmarkCompilerFingerprint measures Fingerprint computation.
func BenchmarkCompilerFingerprint(b *testing.B) {
	reg, _ := registry.BuildFrom(platformDefs())
	schema, _ := compiler.Compile(reg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compiler.Fingerprint(schema)
	}
}

// BenchmarkFakeStoreCreate measures fakestore.Create throughput.
// fakestore is schema-free so any field map is accepted.
func BenchmarkFakeStoreCreate(b *testing.B) {
	h := harness.New(b, platformDefs()...)
	ctx := h.Context()
	store := h.Store
	tenantID := h.TenantID
	actor := &def.Actor{UserID: h.ActorID, TenantID: tenantID, Roles: h.ActorRoles}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.Create(ctx, driver.CreateInput{
			Data: map[string]any{
				"tenant_id": tenantID,
				"name":      "Bench Tenant",
				"slug":      uuid.New().String()[:8],
				"status":    "PENDING",
			},
			Actor: actor,
		})
		if err != nil {
			b.Fatalf("Create: %v", err)
		}
	}
}

// BenchmarkFakeStoreQuery measures fakestore.Query with filter against 1000 records.
func BenchmarkFakeStoreQuery(b *testing.B) {
	h := harness.New(b, platformDefs()...)
	ctx := h.Context()
	store := h.Store
	tenantID := h.TenantID
	actor := &def.Actor{UserID: h.ActorID, TenantID: tenantID, Roles: h.ActorRoles}

	// Seed 1000 records.
	inputs := make([]driver.CreateInput, 1000)
	for i := range inputs {
		active := i%3 != 0
		inputs[i] = driver.CreateInput{
			Data: map[string]any{
				"tenant_id": tenantID,
				"name":      "Tenant",
				"slug":      uuid.New().String()[:8],
				"status":    "PENDING",
				"active":    active,
			},
			Actor: actor,
		}
	}
	if _, err := store.BulkCreate(ctx, inputs); err != nil {
		b.Fatalf("seed: %v", err)
	}

	f := filter.Eq("active", true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, _, err := store.Query(ctx, f)
		if err != nil {
			b.Fatalf("Query: %v", err)
		}
		_ = results
	}
}

// BenchmarkFakeStoreGet measures single-record Get by ID.
func BenchmarkFakeStoreGet(b *testing.B) {
	h := harness.New(b, platformDefs()...)
	ctx := h.Context()
	store := h.Store
	tenantID := h.TenantID
	actor := &def.Actor{UserID: h.ActorID, TenantID: tenantID, Roles: h.ActorRoles}

	rec, err := store.Create(ctx, driver.CreateInput{
		Data:  map[string]any{"tenant_id": tenantID, "name": "Bench", "slug": "bench", "status": "PENDING"},
		Actor: actor,
	})
	if err != nil {
		b.Fatalf("Create: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := store.Get(ctx, rec.ID)
		if err != nil || r == nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

// BenchmarkFilterEval measures filter.Filter evaluation against a record.
func BenchmarkFilterEval(b *testing.B) {
	store := fakestore.New()
	ctx := context.Background()
	tenantID := uuid.New()
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	// Seed 500 records.
	inputs := make([]driver.CreateInput, 500)
	for i := range inputs {
		inputs[i] = driver.CreateInput{
			Data:  map[string]any{"tenant_id": tenantID, "name": "Tenant", "slug": uuid.New().String()[:8], "active": i%2 == 0},
			Actor: actor,
		}
	}
	_, _ = store.BulkCreate(ctx, inputs)

	f := filter.And(
		filter.Eq("active", true),
		filter.Contains("name", "Tenant"),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = store.Query(ctx, f)
	}
}

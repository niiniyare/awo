// Package perf provides performance benchmarking helpers for Awo stores and
// filter evaluation. Use these in *_test.go files with the standard Go
// benchmarking harness.
package perf

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime/tenant"
)

// BenchmarkStore runs a standard suite of benchmarks against a RecordRepository.
// It measures Create, Get, Query (with and without filters), Count, and BulkCreate
// throughput. Results are reported via b.ReportMetric.
//
// Call from a *testing.B function:
//
//	func BenchmarkMyStore(b *testing.B) {
//	    perf.BenchmarkStore(b, func() driver.RecordRepository { return mystore.New() })
//	}
func BenchmarkStore(b *testing.B, factory func() driver.RecordRepository) {
	b.Helper()

	b.Run("Create", func(b *testing.B) {
		store := factory()
		ctx := benchCtx()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := store.Create(ctx, driver.CreateInput{
				Data: map[string]any{"name": fmt.Sprintf("item-%d", i), "status": "active"},
			})
			if err != nil {
				b.Fatalf("Create: %v", err)
			}
		}
	})

	b.Run("Get", func(b *testing.B) {
		store := factory()
		ctx := benchCtx()
		// Pre-populate.
		ids := make([]uuid.UUID, 100)
		for i := range ids {
			r, _ := store.Create(ctx, driver.CreateInput{
				Data: map[string]any{"name": fmt.Sprintf("item-%d", i)},
			})
			ids[i] = r.ID
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id := ids[i%len(ids)]
			if _, err := store.Get(ctx, id); err != nil {
				b.Fatalf("Get: %v", err)
			}
		}
	})

	b.Run("QueryNoFilter", func(b *testing.B) {
		store := factory()
		ctx := benchCtx()
		for i := 0; i < 1000; i++ {
			store.Create(ctx, driver.CreateInput{
				Data: map[string]any{"status": "active"},
			})
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, _, err := store.Query(ctx, nil, driver.WithPage(1, 20)); err != nil {
				b.Fatalf("Query: %v", err)
			}
		}
	})

	b.Run("QueryEqFilter", func(b *testing.B) {
		store := factory()
		ctx := benchCtx()
		for i := 0; i < 1000; i++ {
			status := "active"
			if i%3 == 0 {
				status = "draft"
			}
			store.Create(ctx, driver.CreateInput{
				Data: map[string]any{"status": status},
			})
		}
		f := filter.Eq("status", "active")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, _, err := store.Query(ctx, f, driver.WithPage(1, 20)); err != nil {
				b.Fatalf("Query: %v", err)
			}
		}
	})

	b.Run("Count", func(b *testing.B) {
		store := factory()
		ctx := benchCtx()
		for i := 0; i < 1000; i++ {
			store.Create(ctx, driver.CreateInput{
				Data: map[string]any{"n": int64(i)},
			})
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := store.Count(ctx, nil); err != nil {
				b.Fatalf("Count: %v", err)
			}
		}
	})

	b.Run("BulkCreate100", func(b *testing.B) {
		store := factory()
		ctx := benchCtx()
		inputs := make([]driver.CreateInput, 100)
		for i := range inputs {
			inputs[i] = driver.CreateInput{
				Data: map[string]any{"name": fmt.Sprintf("item-%d", i)},
			}
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := store.BulkCreate(ctx, inputs); err != nil {
				b.Fatalf("BulkCreate: %v", err)
			}
		}
	})
}

func benchCtx() context.Context {
	return tenant.WithContext(context.Background(), tenant.TenantContext{
		TenantID:   uuid.New(),
		TenantSlug: "bench",
		Locale:     "en-KE",
		Timezone:   "Africa/Nairobi",
		Currency:   "KES",
	})
}

package observability_test

import (
	"context"
	"testing"
	"time"

	"awo.so/awo/sdui/observability"
)

func TestNoop_NoPanic(t *testing.T) {
	obs := observability.Noop()
	ctx := context.Background()

	// All calls on a Noop must not panic.
	ctx2, finish := obs.StartSpan(ctx, "test")
	finish()
	_ = ctx2

	timer := obs.TrackGeneration(ctx, "finance_invoice", "list")
	time.Sleep(time.Millisecond)
	timer.Done()

	obs.TrackRender(ctx, "amis").Done()
	obs.TrackValidation(ctx).Done()
	obs.TrackLayout(ctx).Done()
	obs.TrackPlugins(ctx, "post_generation").Done()

	obs.RecordCacheHit(ctx, observability.CacheLevelL2)
	obs.RecordCacheMiss(ctx, observability.CacheLevelL3)
	obs.RecordError(ctx, observability.StageGenerate)
}

func TestNoop_TimerMeasures(t *testing.T) {
	obs := observability.Noop()
	ctx := context.Background()
	// Timer should not block or panic even without a real meter.
	start := time.Now()
	timer := obs.TrackGeneration(ctx, "test", "list")
	time.Sleep(5 * time.Millisecond)
	timer.Done()
	if time.Since(start) < 5*time.Millisecond {
		t.Error("timer completed before sleep ended")
	}
}

func TestNew_GlobalProvider_NoPanic(t *testing.T) {
	// OTel global provider is a no-op by default — New should succeed.
	obs, err := observability.New(observability.Config{
		MeterName:  "awo.sdui.test",
		TracerName: "awo.sdui.test",
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()
	obs.RecordCacheHit(ctx, observability.CacheLevelL2)
	obs.RecordCacheMiss(ctx, observability.CacheLevelL3)
	obs.RecordError(ctx, observability.StageRender)
	obs.TrackLayout(ctx).Done()
}

func TestNew_DefaultNames(t *testing.T) {
	// Empty config uses default names — must not error.
	_, err := observability.New(observability.Config{})
	if err != nil {
		t.Fatalf("New with default config returned error: %v", err)
	}
}

func BenchmarkTimer_Done(b *testing.B) {
	obs := observability.Noop()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obs.TrackGeneration(ctx, "entity", "list").Done()
	}
}

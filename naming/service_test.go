package naming

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/cache"
)

// --- atomic in-process counter for tests (no Redis needed) ---

type atomicCounter struct {
	mu      sync.Mutex
	buckets map[string]int64
}

func newAtomicCounter() *atomicCounter {
	return &atomicCounter{buckets: make(map[string]int64)}
}

func (c *atomicCounter) Increment(_ context.Context, key string, by int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buckets[key] += by
	return c.buckets[key], nil
}

func (c *atomicCounter) IncrementWithReset(_ context.Context, key string, by int64, _ time.Duration) (int64, error) {
	return c.Increment(context.Background(), key, by)
}

// Ensure atomicCounter satisfies cache.Counter.
var _ cache.Counter = (*atomicCounter)(nil)

// --- NamingSeriesService tests ---

func TestNamingSeriesService_Allocate_FirstCall(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	id, seq, err := svc.Allocate(context.Background(), AllocateInput{
		TenantID: uuid.New(),
		Pattern:  "INV-{YYYY}-{SEQ:6}",
		Now:      refTime,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), seq)
	assert.Equal(t, "INV-2026-000001", id)
}

func TestNamingSeriesService_Allocate_Increments(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	tenant := uuid.New()

	for i := int64(1); i <= 5; i++ {
		id, seq, err := svc.Allocate(context.Background(), AllocateInput{
			TenantID: tenant,
			Pattern:  "PAY-{YYYY}-{SEQ:6}",
			Now:      refTime,
		})
		require.NoError(t, err)
		assert.Equal(t, i, seq)
		assert.Contains(t, id, "PAY-2026-")
	}
}

func TestNamingSeriesService_Allocate_TenantIsolation(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	t1 := uuid.New()
	t2 := uuid.New()

	_, seq1, err := svc.Allocate(context.Background(), AllocateInput{TenantID: t1, Pattern: "JE-{SEQ:5}", Now: refTime})
	require.NoError(t, err)
	_, seq2, err := svc.Allocate(context.Background(), AllocateInput{TenantID: t2, Pattern: "JE-{SEQ:5}", Now: refTime})
	require.NoError(t, err)

	// Both tenants start their own sequence at 1 — counter keys are different.
	assert.Equal(t, int64(1), seq1)
	assert.Equal(t, int64(1), seq2)
}

func TestNamingSeriesService_Allocate_InvalidPattern(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	_, _, err := svc.Allocate(context.Background(), AllocateInput{
		TenantID: uuid.New(),
		Pattern:  "{INVALID}",
	})
	require.Error(t, err)
}

func TestNamingSeriesService_Preview(t *testing.T) {
	svc := NewNamingSeriesService(cache.NoopCounter{})
	preview, err := svc.Preview("INV-{YYYY}-{SEQ:6}", "", refTime)
	require.NoError(t, err)
	assert.Equal(t, "INV-2026-000001", preview)
}

func TestNamingSeriesService_Validate_Valid(t *testing.T) {
	svc := NewNamingSeriesService(cache.NoopCounter{})
	assert.NoError(t, svc.Validate("INV-{YYYY}-{SEQ:6}"))
}

func TestNamingSeriesService_Validate_Invalid(t *testing.T) {
	svc := NewNamingSeriesService(cache.NoopCounter{})
	assert.Error(t, svc.Validate("{BOGUS}"))
}

func TestNamingSeriesService_TokenCaching(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	tenant := uuid.New()
	pattern := "INV-{YYYY}-{SEQ:6}"

	// First call parses and caches; second call hits the cache.
	for i := 0; i < 3; i++ {
		_, _, err := svc.Allocate(context.Background(), AllocateInput{
			TenantID: tenant,
			Pattern:  pattern,
			Now:      refTime,
		})
		require.NoError(t, err, "iteration %d should not error", i)
	}
}

func TestNamingSeriesService_Allocate_Concurrent(t *testing.T) {
	counter := newAtomicCounter()
	svc := NewNamingSeriesService(counter)
	tenant := uuid.New()
	pattern := "INV-{YYYY}-{SEQ:6}"

	const goroutines = 20
	ids := make([]string, goroutines)
	var wg sync.WaitGroup
	var failures int32

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			id, _, err := svc.Allocate(context.Background(), AllocateInput{
				TenantID: tenant,
				Pattern:  pattern,
				Now:      refTime,
			})
			if err != nil {
				atomic.AddInt32(&failures, 1)
				return
			}
			ids[idx] = id
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int32(0), failures, "no goroutine should fail")

	// All allocated IDs must be unique (no duplicate sequence numbers).
	seen := make(map[string]bool, goroutines)
	for _, id := range ids {
		if id == "" {
			continue // goroutine failed
		}
		assert.False(t, seen[id], "duplicate ID detected: %s", id)
		seen[id] = true
	}
}

func TestNamingSeriesService_AllocateForRecord(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	id, err := svc.AllocateForRecord(context.Background(), NamingFieldContext{
		FieldName: "invoice_number",
		Pattern:   "INV-{YYYY}-{SEQ:6}",
		TenantID:  uuid.New(),
	})
	require.NoError(t, err)
	assert.Contains(t, id, "INV-")
}

func TestNamingSeriesService_AllocateForRecord_InvalidPattern(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	_, err := svc.AllocateForRecord(context.Background(), NamingFieldContext{
		FieldName: "invoice_number",
		Pattern:   "{BAD}",
		TenantID:  uuid.New(),
	})
	require.Error(t, err)
}

// TestNamingSeriesService_Allocate_ZeroNowUsesCurrentTime verifies that when
// AllocateInput.Now is zero, the service uses the current UTC time without
// panicking.
func TestNamingSeriesService_Allocate_ZeroNowUsesCurrentTime(t *testing.T) {
	svc := NewNamingSeriesService(newAtomicCounter())
	id, _, err := svc.Allocate(context.Background(), AllocateInput{
		TenantID: uuid.New(),
		Pattern:  "INV-{YYYY}-{SEQ:6}",
		Now:      time.Time{}, // zero → uses time.Now()
	})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

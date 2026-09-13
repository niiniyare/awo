package events_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/observability/events"
)

func TestBus_Subscribe_Emit(t *testing.T) {
	bus := events.NewBus()
	var got events.Event
	bus.Subscribe(events.KindCompilerCompleted, func(e events.Event) { got = e })

	bus.CompilerCompleted("abc123", 7)

	assert.Equal(t, events.KindCompilerCompleted, got.Kind)
	assert.Equal(t, "abc123", got.Data["fingerprint"])
	assert.Equal(t, 7, got.Data["entity_count"])
	assert.False(t, got.OccurredAt.IsZero())
}

func TestBus_SubscribeAll(t *testing.T) {
	bus := events.NewBus()
	var kinds []events.Kind
	bus.SubscribeAll(func(e events.Event) { kinds = append(kinds, e.Kind) })

	bus.FrameworkStarted("awo", "1.0.0")
	bus.FrameworkStopped("awo")
	bus.CompilerCompleted("fp", 3)

	require.Len(t, kinds, 3)
	assert.Equal(t, events.KindFrameworkStarted, kinds[0])
	assert.Equal(t, events.KindFrameworkStopped, kinds[1])
	assert.Equal(t, events.KindCompilerCompleted, kinds[2])
}

func TestBus_MultipleHandlers(t *testing.T) {
	bus := events.NewBus()
	count := 0
	bus.Subscribe(events.KindMigrationApplied, func(events.Event) { count++ })
	bus.Subscribe(events.KindMigrationApplied, func(events.Event) { count++ })

	bus.MigrationApplied("001_create.up.sql", 1)
	assert.Equal(t, 2, count)
}

func TestBus_OccurredAt_SetWhenZero(t *testing.T) {
	bus := events.NewBus()
	before := time.Now()
	var got events.Event
	bus.Subscribe(events.KindFrameworkStarted, func(e events.Event) { got = e })

	bus.Emit(events.Event{Kind: events.KindFrameworkStarted})

	assert.False(t, got.OccurredAt.IsZero())
	assert.True(t, got.OccurredAt.After(before) || got.OccurredAt.Equal(before))
}

func TestBus_OccurredAt_PreservedWhenSet(t *testing.T) {
	bus := events.NewBus()
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var got events.Event
	bus.Subscribe(events.KindFrameworkStarted, func(e events.Event) { got = e })

	bus.Emit(events.Event{Kind: events.KindFrameworkStarted, OccurredAt: fixed})
	assert.Equal(t, fixed, got.OccurredAt)
}

func TestBus_CompilerFailed(t *testing.T) {
	bus := events.NewBus()
	var got events.Event
	bus.Subscribe(events.KindCompilerFailed, func(e events.Event) { got = e })

	bus.CompilerFailed(errors.New("circular dependency"))
	assert.Equal(t, "circular dependency", got.Data["error"])
}

func TestBus_MigrationFailed(t *testing.T) {
	bus := events.NewBus()
	var got events.Event
	bus.Subscribe(events.KindMigrationFailed, func(e events.Event) { got = e })

	bus.MigrationFailed("005_drop_table.up.sql", 5, errors.New("syntax error"))
	assert.Equal(t, "005_drop_table.up.sql", got.Data["filename"])
	assert.Equal(t, uint(5), got.Data["version"])
	assert.Equal(t, "syntax error", got.Data["error"])
}

func TestBus_ConcurrentEmit(t *testing.T) {
	bus := events.NewBus()
	var mu sync.Mutex
	count := 0
	bus.Subscribe(events.KindMigrationApplied, func(events.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			bus.MigrationApplied("file", uint(i))
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 50, count)
}

func TestBus_StartupDiagnostic(t *testing.T) {
	bus := events.NewBus()
	var got events.Event
	bus.Subscribe(events.KindStartupDiagnostic, func(e events.Event) { got = e })

	bus.StartupDiagnostic("money field uses FieldTypeInt", "warning")
	assert.Equal(t, "warning", got.Data["severity"])
}

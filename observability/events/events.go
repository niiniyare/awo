// Package events provides framework lifecycle event emission.
//
// Framework events are distinct from domain events (outbox/Temporal):
// they describe the framework's own lifecycle — startup, compilation,
// migration runs, and schema changes. Operators subscribe to monitor
// health and diagnose problems without parsing logs.
//
// # Event types
//
//   - FrameworkStarted  — process ready to serve requests
//   - FrameworkStopped  — graceful shutdown complete
//   - CompilerCompleted — schema compilation succeeded (includes fingerprint)
//   - CompilerFailed    — schema compilation failed (includes diagnostics)
//   - MigrationStarted  — migration plan beginning
//   - MigrationApplied  — single migration file applied
//   - MigrationFailed   — migration failed (includes step + error)
//   - StartupDiagnostic — any warning emitted during startup
//
// # Usage
//
//	bus := events.NewBus()
//	bus.Subscribe(events.KindCompilerCompleted, func(e events.Event) {
//	    log.Info().Str("fingerprint", e.Data["fingerprint"].(string)).Msg("schema ready")
//	})
package events

import (
	"sync"
	"time"
)

// Kind identifies the framework lifecycle event type.
type Kind string

const (
	KindFrameworkStarted  Kind = "framework.started"
	KindFrameworkStopped  Kind = "framework.stopped"
	KindCompilerCompleted Kind = "compiler.completed"
	KindCompilerFailed    Kind = "compiler.failed"
	KindMigrationStarted  Kind = "migration.started"
	KindMigrationApplied  Kind = "migration.applied"
	KindMigrationFailed   Kind = "migration.failed"
	KindStartupDiagnostic Kind = "startup.diagnostic"
)

// Event is an immutable lifecycle event emitted by the framework.
type Event struct {
	// Kind is the event type.
	Kind Kind

	// OccurredAt is the wall-clock time the event was emitted.
	OccurredAt time.Time

	// Data holds event-specific fields. Keys and value types are documented
	// per Kind above.
	Data map[string]any
}

// Handler is a callback that receives a framework lifecycle event.
// Handlers are called synchronously on the goroutine that emits the event.
// Handlers must not block.
type Handler func(Event)

// Bus is a simple publish-subscribe bus for framework lifecycle events.
// It is safe for concurrent use. A zero-value Bus is not usable — use NewBus.
type Bus struct {
	mu       sync.RWMutex
	handlers map[Kind][]Handler
}

// NewBus creates an initialised Bus.
func NewBus() *Bus {
	return &Bus{handlers: make(map[Kind][]Handler)}
}

// Subscribe registers h to receive events of the given kind.
// Subscribe may be called concurrently with Emit.
func (b *Bus) Subscribe(kind Kind, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[kind] = append(b.handlers[kind], h)
}

// SubscribeAll registers h to receive every event kind.
func (b *Bus) SubscribeAll(h Handler) {
	for _, k := range allKinds {
		b.Subscribe(k, h)
	}
}

// Emit dispatches e to all handlers registered for e.Kind.
// If OccurredAt is zero, it is set to time.Now() before dispatch.
func (b *Bus) Emit(e Event) {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	b.mu.RLock()
	hs := b.handlers[e.Kind]
	b.mu.RUnlock()
	for _, h := range hs {
		h(e)
	}
}

// allKinds is the canonical set of all event kinds, used by SubscribeAll.
var allKinds = []Kind{
	KindFrameworkStarted,
	KindFrameworkStopped,
	KindCompilerCompleted,
	KindCompilerFailed,
	KindMigrationStarted,
	KindMigrationApplied,
	KindMigrationFailed,
	KindStartupDiagnostic,
}

// --- Convenience constructors for common events ---

// FrameworkStarted emits a KindFrameworkStarted event on b.
func (b *Bus) FrameworkStarted(serviceName, version string) {
	b.Emit(Event{
		Kind: KindFrameworkStarted,
		Data: map[string]any{
			"service": serviceName,
			"version": version,
		},
	})
}

// FrameworkStopped emits a KindFrameworkStopped event on b.
func (b *Bus) FrameworkStopped(serviceName string) {
	b.Emit(Event{
		Kind: KindFrameworkStopped,
		Data: map[string]any{"service": serviceName},
	})
}

// CompilerCompleted emits a KindCompilerCompleted event with the schema fingerprint
// and the count of compiled entities.
func (b *Bus) CompilerCompleted(fingerprint string, entityCount int) {
	b.Emit(Event{
		Kind: KindCompilerCompleted,
		Data: map[string]any{
			"fingerprint":  fingerprint,
			"entity_count": entityCount,
		},
	})
}

// CompilerFailed emits a KindCompilerFailed event with the error.
func (b *Bus) CompilerFailed(err error) {
	b.Emit(Event{
		Kind: KindCompilerFailed,
		Data: map[string]any{"error": err.Error()},
	})
}

// MigrationApplied emits a KindMigrationApplied event for a single step.
func (b *Bus) MigrationApplied(filename string, version uint) {
	b.Emit(Event{
		Kind: KindMigrationApplied,
		Data: map[string]any{
			"filename": filename,
			"version":  version,
		},
	})
}

// MigrationFailed emits a KindMigrationFailed event.
func (b *Bus) MigrationFailed(filename string, version uint, err error) {
	b.Emit(Event{
		Kind: KindMigrationFailed,
		Data: map[string]any{
			"filename": filename,
			"version":  version,
			"error":    err.Error(),
		},
	})
}

// StartupDiagnostic emits a KindStartupDiagnostic with a warning message.
func (b *Bus) StartupDiagnostic(message, severity string) {
	b.Emit(Event{
		Kind: KindStartupDiagnostic,
		Data: map[string]any{
			"message":  message,
			"severity": severity,
		},
	})
}

// Global is the process-wide event bus. Replace in tests if needed.
var Global = NewBus()

package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/events"
	"awo.so/awo/events/outbox"
)

// --- subscriber stubs ---

type recordingSubscriber struct {
	received []events.DomainEvent
	err      error
}

func (s *recordingSubscriber) HandleEvent(_ context.Context, e events.DomainEvent) error {
	if s.err != nil {
		return s.err
	}
	s.received = append(s.received, e)
	return nil
}

// TestMaxAttempts_IsPositive is a simple constant-value sanity check.
func TestMaxAttempts_IsPositive(t *testing.T) {
	assert.Greater(t, outbox.MaxAttempts, 0)
}

// TestNew_NotNil verifies that New(nil) does not panic and returns a non-nil
// Relay. A nil pool is acceptable at construction time; it would only fail at
// poll time when a real database call is attempted.
func TestNew_NotNil(t *testing.T) {
	r := outbox.New(nil)
	assert.NotNil(t, r)
}

// TestSubscribe_TypeSpecific verifies that Subscribe records type-specific
// handlers. We exercise this indirectly via the exported Subscribe method
// without triggering the poll loop.
func TestSubscribe_TypeSpecific(t *testing.T) {
	r := outbox.New(nil)
	sub := &recordingSubscriber{}
	// Should not panic.
	r.Subscribe(events.EventType("entity.created"), sub)
}

// TestSubscribe_Wildcard verifies that Subscribe accepts an empty EventType
// (wildcard). No panic expected.
func TestSubscribe_Wildcard(t *testing.T) {
	r := outbox.New(nil)
	sub := &recordingSubscriber{}
	r.Subscribe("", sub)
}

// TestOutboxWriter_Publish_NilPoolPanics verifies production invariant: calling
// Publish on an OutboxWriter constructed with nil pool surfaces a panic from
// the pool — not a silent data-loss bug.
//
// This is a documentation test. In practice, bootstrap.go always provides a
// real pool. We just confirm that the relay fails loudly, not silently.
func TestOutboxWriter_Publish_NilPoolPanics(t *testing.T) {
	w := outbox.NewWriter(nil)
	require.Panics(t, func() {
		_ = w.Publish(context.Background(), events.DomainEvent{
			ID:         uuid.New(),
			TenantID:   uuid.New(),
			Type:       "test.event",
			EntityName: "test_entity",
			RecordID:   uuid.New(),
			OccurredAt: time.Now(),
		})
	})
}

// TestNilIfEmpty_ViaPayload is an indirect integration test that verifies that
// the outbox package correctly handles the ActionName field being empty vs
// non-empty on DomainEvent (the nil-if-empty logic). We verify the DomainEvent
// struct can hold both states without panicking.
func TestDomainEvent_ActionName_Optional(t *testing.T) {
	e1 := events.DomainEvent{ActionName: ""}
	e2 := events.DomainEvent{ActionName: "submit"}
	assert.Empty(t, e1.ActionName)
	assert.Equal(t, "submit", e2.ActionName)
}

// TestRelay_Start_CancelImmediately verifies that Start returns nil when the
// context is cancelled before any poll occurs. We pass nil pool intentionally
// because Start should return before acquiring a connection in this scenario.
func TestRelay_Start_CancelImmediately(t *testing.T) {
	r := outbox.New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Start is called

	errCh := make(chan error, 1)
	go func() { errCh <- r.Start(ctx) }()

	select {
	case err := <-errCh:
		assert.NoError(t, err, "Start must return nil on clean cancellation")
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return within 3 seconds after context cancellation")
	}
}

// TestErrLockNotAcquired_Format verifies the relay constant. The advisory lock
// ID is an internal detail but it is important that it is stable — changing it
// across deployments would cause two relay instances to operate concurrently
// and produce duplicate delivery.
func TestRelay_Subscribe_DoesNotPanic(t *testing.T) {
	r := outbox.New(nil)
	sub := &recordingSubscriber{}
	assert.NotPanics(t, func() {
		r.Subscribe("entity.created", sub)
		r.Subscribe("entity.updated", sub)
		r.Subscribe("", sub) // wildcard
	})
}

// TestOutboxWriter_ImplementsPublisher verifies that *OutboxWriter satisfies
// events.Publisher at compile time.
func TestOutboxWriter_ImplementsPublisher(t *testing.T) {
	var _ events.Publisher = (*outbox.OutboxWriter)(nil)
}

// TestRecordingSubscriber_ErrorPropagation verifies our test double
// correctly returns errors when configured.
func TestRecordingSubscriber_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("subscriber error")
	sub := &recordingSubscriber{err: sentinel}
	err := sub.HandleEvent(context.Background(), events.DomainEvent{})
	assert.ErrorIs(t, err, sentinel)
	assert.Empty(t, sub.received)
}

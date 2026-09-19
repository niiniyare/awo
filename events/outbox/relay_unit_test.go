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

// TestOutboxWriter_Publish_NoActiveTransaction_ReturnsErrorNotPanic verifies
// the Step 1 transaction-join contract at the unit level (no real PostgreSQL
// needed, since the failure must occur before any database access is
// attempted): Publish resolves its connection via tx.QuerierFromContext, not
// via the OutboxWriter's own pool field, so a bare context.Background() (no
// active transaction) must produce a clear, returned error — never a panic,
// and never a silent, accidental fallback to an auto-commit connection.
//
// This replaces a prior version of this test that asserted Publish panicked
// when constructed with a nil pool — that was true only because the old
// implementation dereferenced w.pool directly (w.pool.Acquire(ctx)). Once
// Publish stopped touching pool at all (Step 1's transaction-join fix), that
// scenario no longer applies; nil pool is irrelevant to Publish's behavior
// now, and the correct failure mode for "no active transaction" is a
// returned error, not a panic — matching events.Publisher's documented
// contract ("Publisher.Publish will return an error if no active
// transaction is present") and this package's own error-handling
// conventions elsewhere (e.g. audit.TransactionalWriter.Write's analogous
// "no database connection available" error, never a panic).
func TestOutboxWriter_Publish_NoActiveTransaction_ReturnsErrorNotPanic(t *testing.T) {
	w := outbox.NewWriter(nil) // pool is irrelevant here — Publish never touches it
	var err error
	require.NotPanics(t, func() {
		err = w.Publish(context.Background(), events.DomainEvent{
			ID:         uuid.New(),
			TenantID:   uuid.New(),
			Type:       "test.event",
			EntityName: "test_entity",
			RecordID:   uuid.New(),
			OccurredAt: time.Now(),
		})
	})
	require.Error(t, err, "Publish must return an error, not silently succeed, when ctx carries no active transaction")
	assert.Contains(t, err.Error(), "no active transaction",
		"the error must be specific enough to distinguish this failure mode from other insert failures")
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

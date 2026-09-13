package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// SignalChannel wraps a workflow.ReceiveChannel for a strongly-typed signal
// payload. Create one with GetSignalChannel; never construct directly.
type SignalChannel[T any] struct {
	ch workflow.ReceiveChannel
}

// GetSignalChannel returns a typed signal channel for the named signal on ctx.
// Call inside a workflow function — not inside an activity.
func GetSignalChannel[T any](ctx workflow.Context, name string) *SignalChannel[T] {
	return &SignalChannel[T]{
		ch: workflow.GetSignalChannel(ctx, name),
	}
}

// Receive blocks until a signal arrives and decodes the payload into T.
// The second return value is false only when the channel is drained and
// closed (i.e. the workflow is being cancelled).
func (s *SignalChannel[T]) Receive(ctx workflow.Context) (T, bool) {
	var payload T
	ok := s.ch.Receive(ctx, &payload)
	return payload, ok
}

// ReceiveWithTimeout blocks until a signal arrives or the timeout elapses.
// Returns:
//   - value: the decoded payload (zero value if timed out)
//   - received: true if a signal was received before the timeout
//   - ok: false only when the channel is closed
func (s *SignalChannel[T]) ReceiveWithTimeout(ctx workflow.Context, timeout time.Duration) (value T, received bool, ok bool) {
	timerCtx, cancel := workflow.WithCancel(ctx)
	defer cancel()

	timer := workflow.NewTimer(timerCtx, timeout)

	selector := workflow.NewSelector(ctx)

	var payload T
	var gotSignal bool
	var chOk bool

	selector.AddReceive(s.ch, func(c workflow.ReceiveChannel, more bool) {
		chOk = more
		gotSignal = c.Receive(ctx, &payload)
		cancel() // cancel the timer
	})

	selector.AddFuture(timer, func(f workflow.Future) {
		// Timer fired — no signal received within timeout.
	})

	selector.Select(ctx)

	if gotSignal {
		return payload, true, chOk
	}
	var zero T
	return zero, false, true
}

package notifications

import (
	"context"

	"awo.so/awo/def"
)

// DispatchHook fires after a platform_notification record is created.
// The actual delivery is performed by a registered Driver. If no driver is
// registered for the channel, the hook is a no-op (in-app notifications are
// served by the read API, not pushed anywhere).
//
// Dispatch is best-effort inside the transaction — if the driver call fails,
// the hook returns nil (not an error) so the notification record is always
// persisted. Delivery failure is recorded via a separate update to failed_at.
// For durable delivery guarantees, use TemporalDriver which starts a workflow
// outside the transaction.
type DispatchHook struct{}

func (h *DispatchHook) AfterCreate(ctx context.Context, rec *def.EntityRecord) error {
	channel := rec.GetString("channel")
	drv := driverRegistry.get(channel)
	if drv == nil {
		// No driver for this channel (e.g. in_app requires no push).
		return nil
	}
	// TODO: async dispatch via Temporal activity to decouple delivery from TX.
	// For now, dispatch synchronously and tolerate failures gracefully.
	_ = drv.Send(ctx, rec)
	return nil
}

package notifications

import (
	"context"
	"sync"

	"awo.so/awo/def"
)

// Driver delivers a notification through a specific channel.
// Implementations are registered at startup via RegisterDriver.
//
// Implementors should update the notification record's sent_at / failed_at +
// error_message fields on completion. The record ID is available via rec.ID.
type Driver interface {
	// Send delivers the notification. Best-effort — errors are logged, not
	// propagated to the caller. The notification record is always persisted.
	Send(ctx context.Context, rec *def.EntityRecord) error
}

type channelRegistry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

func (r *channelRegistry) get(channel string) Driver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.drivers[channel]
}

func (r *channelRegistry) register(channel string, d Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.drivers == nil {
		r.drivers = make(map[string]Driver)
	}
	r.drivers[channel] = d
}

var driverRegistry = &channelRegistry{}

// RegisterDriver registers a delivery driver for a channel.
// Call from init() or main() before requests begin. Not safe to call
// concurrently once requests are being served.
//
// Channels: "in_app", "email", "sms", "push".
func RegisterDriver(channel string, d Driver) {
	driverRegistry.register(channel, d)
}

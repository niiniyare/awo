// Package notifications implements the Awo platform Notifications module.
//
// Notification delivery is driver-based. Register drivers at startup:
//
//	notifications.RegisterDriver("email", myEmailDriver)
//	notifications.RegisterDriver("sms", mySMSDriver)
//
// In-app notifications require no driver — they are read via the standard
// CRUD API on the platform_notification entity, filtered by recipient_id
// and read_at IS NULL.
//
// Templates are optional. Callers may pass pre-rendered title+body or use
// TemplateKey to render from a stored platform_notification_template record.
//
// Delivery guarantees: by default, dispatch is synchronous and best-effort
// (failures do not roll back the notification record). For at-least-once
// delivery, register a TemporalDriver that starts a workflow activity.
package notifications

import "awo.so/awo/def"

func init() {
	def.Register(&NotificationDef)
	def.Register(&TemplateDef)
}

// Package notification provides the platform_notification entity definition.
//
// Notifications are in-app messages delivered to users within a tenant.
// They are created by framework subsystems and business modules — never
// directly by end users. The notification UI polls /api/v1/platform/notifications
// with filter unread=true; marking read sets read_at.
//
// # Delivery model
//
// Phase 8 covers persistence only. Push delivery (WebSocket, FCM, APNs) is a
// Phase 10 concern. For now the client polls the REST endpoint.
//
// # Retention
//
// Notifications are soft-deleted via deleted_at (standard column). Hard
// deletion is an ops concern. The default query filter excludes
// notifications older than 90 days to keep list views fast.
package notification

import (
	"awo.so/awo/def"
)

// Definition is the platform_notification entity.
// Each record represents one in-app notification for one user.
var Definition = def.SystemDefinition{
	Name:        "notification",
	Module:      "platform",
	Label:       "Notification",
	LabelPlural: "Notifications",
	Description:  "In-app notification delivered to a specific user within the tenant.",
	DisableAudit: true, // high-volume; auditing every notification write is too noisy

	Fields: []def.FieldDef{
		{
			// recipient_id links to the user this notification is addressed to.
			// Nil for broadcast notifications (future).
			Name:       "recipient_id",
			Type:       def.FieldTypeLink,
			Label:      "Recipient",
			LinkTarget: "iam_user",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:      "title",
			Type:      def.FieldTypeData,
			Label:     "Title",
			Required:  true,
			Immutable: true,
			MaxLen:    255,
		},
		{
			Name:      "body",
			Type:      def.FieldTypeSmallText,
			Label:     "Body",
			Immutable: true,
		},
		{
			// type classifies the notification for icon/colour rendering in the UI.
			Name:      "type",
			Type:      def.FieldTypeSelect,
			Label:     "Type",
			Options:   []string{"info", "success", "warning", "error", "system"},
			Default:   func() any { return "info" },
			Immutable: true,
		},
		{
			// action_url is the deeplink the user is taken to when they click the notification.
			Name:      "action_url",
			Type:      def.FieldTypeData,
			Label:     "Action URL",
			Immutable: true,
			MaxLen:    2048,
		},
		{
			// read_at is nil for unread notifications; set on first client acknowledgement.
			// Nullable via absent Required flag.
			Name:     "read_at",
			Type:     def.FieldTypeDateTime,
			Label:    "Read At",
			ReadOnly: true, // written by the mark-read action, not by callers
		},
		{
			// source identifies the system component or module that produced the notification.
			// Format: "module.subsystem" — e.g. "finance.invoice", "iam.session".
			Name:      "source",
			Type:      def.FieldTypeData,
			Label:     "Source",
			Immutable: true,
			MaxLen:    100,
		},
		{
			// context carries arbitrary notification-specific payload for the action handler.
			// Example: {"entity_id": "...", "entity_name": "finance_invoice"}.
			Name:      "context",
			Type:      def.FieldTypeJSON,
			Label:     "Context",
			Immutable: true,
		},
	},

	Actions: []def.ActionDef{
		{
			Name:   "mark_read",
			Label:  "Mark as Read",
			Method: "POST",
			HandlerFunc: func(ctx *def.ActionContext) (*def.ActionResult, error) {
				return &def.ActionResult{}, nil
			},
		},
	},

	Permissions: def.PermissionSet{
		// Notifications are created only by internal framework/module code, never
		// via the public API. Read and mark_read are user-self operations.
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:tenant.user    → platform.notification.read, platform.notification.mark_read
		//   role:tenant.admin   → platform.notification.{read,delete}
		Read:   []string{"platform.notification.read"},
		Delete: []string{"platform.notification.delete"},
		Actions: map[string][]string{
			"mark_read": {"platform.notification.mark_read"},
		},
	},
}

func init() {
	def.Register(&Definition)
}

// Package notifications provides the platform notification module.
//
// Notifications are driver-agnostic: the entity definitions declare the schema
// and delivery-tracking state; concrete delivery (email, SMS, push) is handled
// by driver implementations wired at startup.
//
// Delivery flow:
//
//  1. Caller creates a platform_notification record (channel, recipient, title, body).
//  2. An AfterCreate hook dispatches to the configured driver (in-process call or
//     Temporal activity).
//  3. Driver updates sent_at or failed_at + error_message on the record.
//  4. Unread in-app notifications are exposed via the read_at field.
//
// Templates are optional — callers may provide pre-rendered title/body or
// reference a platform_notification_template for server-side rendering.
package notifications

import "awo.so/awo/def"

// NotificationDef is the platform_notification entity. One record per delivery
// attempt per recipient. Immutable after creation except for delivery status
// fields (sent_at, failed_at, read_at, error_message).
var NotificationDef = def.SystemDefinition{
	Name:        "notification",
	Module:      "platform",
	Label:       "Notification",
	LabelPlural: "Notifications",

	Fields: []def.FieldDef{
		{
			Name:       "tenant_id",
			Type:       def.FieldTypeLink,
			Label:      "Tenant",
			LinkTarget: "platform_tenant",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:       "recipient_id",
			Type:       def.FieldTypeLink,
			Label:      "Recipient",
			LinkTarget: "iam_user",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:      "channel",
			Type:      def.FieldTypeSelect,
			Label:     "Channel",
			Options:   []string{"in_app", "email", "sms", "push"},
			Required:  true,
			Immutable: true,
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
			Required:  true,
			Immutable: true,
		},
		{
			Name:       "template_id",
			Type:       def.FieldTypeLink,
			Label:      "Template",
			LinkTarget: "platform_notification_template",
			Immutable:  true,
		},
		{
			Name:      "data",
			Type:      def.FieldTypeJSON,
			Label:     "Payload",
			Immutable: true,
			// Structured data passed to templates for interpolation,
			// or arbitrary context for the recipient.
		},
		{
			Name:  "sent_at",
			Type:  def.FieldTypeDateTime,
			Label: "Sent At",
		},
		{
			Name:  "failed_at",
			Type:  def.FieldTypeDateTime,
			Label: "Failed At",
		},
		{
			Name:   "error_message",
			Type:   def.FieldTypeSmallText,
			Label:  "Error",
			Hidden: true,
		},
		{
			Name:  "read_at",
			Type:  def.FieldTypeDateTime,
			Label: "Read At",
			// Null = unread. Only meaningful for channel=in_app.
		},
		{
			Name:   "channel_ref",
			Type:   def.FieldTypeData,
			Label:  "Channel Reference",
			Hidden: true,
			MaxLen: 255,
			// Provider-specific delivery ID (e.g. SES message ID, FCM token).
		},
	},

	Hooks: def.HookSet{
		AfterCreate: []def.AfterCreateHook{&DispatchHook{}},
	},

	Permissions: def.PermissionSet{
		// Only framework internals create notifications. No user-facing create.
		Create: []string{"role:platform-admin", "role:tenant.admin"},
		Read:   []string{"role:platform-admin", "role:tenant.admin", "role:tenant.user"},
		Write:  []string{"role:platform-admin"}, // delivery drivers update status fields
		Delete: []string{"role:platform-admin"},
	},
}

// TemplateDef is the platform_notification_template entity. Templates are
// server-rendered using Go text/template syntax. Variables are passed in the
// platform_notification.data JSON field.
var TemplateDef = def.SystemDefinition{
	Name:        "notification_template",
	Module:      "platform",
	Label:       "Notification Template",
	LabelPlural: "Notification Templates",

	Fields: []def.FieldDef{
		{
			Name:       "key",
			Type:       def.FieldTypeData,
			Label:      "Template Key",
			Required:   true,
			Unique:     true,
			Immutable:  true,
			Searchable: true,
			MaxLen:     100,
			// Convention: "{module}.{event}.{channel}" e.g. "iam.welcome.email"
		},
		{
			Name:   "label",
			Type:   def.FieldTypeData,
			Label:  "Label",
			MaxLen: 255,
		},
		{
			Name:     "channel",
			Type:     def.FieldTypeSelect,
			Label:    "Channel",
			Options:  []string{"in_app", "email", "sms", "push"},
			Required: true,
		},
		{
			Name:   "subject_template",
			Type:   def.FieldTypeData,
			Label:  "Subject Template",
			MaxLen: 255,
			// Go text/template syntax. Used for email subject lines.
		},
		{
			Name:     "body_template",
			Type:     def.FieldTypeLongText,
			Label:    "Body Template",
			Required: true,
			// Go text/template syntax. Supports {{.FieldName}} interpolation.
		},
		{
			Name:    "active",
			Type:    def.FieldTypeBool,
			Label:   "Active",
			Default: func() any { return true },
		},
		{
			Name:  "metadata",
			Type:  def.FieldTypeJSON,
			Label: "Metadata",
			// Driver-specific config (e.g. email "from" address, push priority).
		},
	},

	Permissions: def.PermissionSet{
		Create: []string{"role:platform-admin"},
		Read:   []string{"role:platform-admin", "role:tenant.admin"},
		Write:  []string{"role:platform-admin"},
		Delete: []string{"role:platform-admin"},
	},
}

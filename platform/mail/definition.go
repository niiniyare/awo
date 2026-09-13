// Package mail provides the platform_mail_record entity definition.
//
// Mail records are the durable log of every email sent by the platform.
// The sending itself is handled by a mail adapter (SMTP, SendGrid, SES) in
// Phase 10; this package owns only the entity definition and migration.
//
// # Record lifecycle
//
// queued → sending → sent
//       ↘ failed
//
// All transitions are written by the mail worker (Phase 10). The record is
// created in the queued state when the caller invokes the mail service.
// Retries increment attempt_count; a maximum of 3 attempts is enforced by
// the worker. Failures are logged in error_message for debugging.
//
// # Content retention
//
// The rendered HTML body is stored for auditability. Sensitive content
// (e.g. password reset links) should use short TTLs on the actual link rather
// than omitting the body from the record.
package mail

import (
	"awo.so/awo/def"
)

// RecordDefinition is the platform_mail_record entity.
// Each record represents one outbound email, queued or sent.
var RecordDefinition = def.SystemDefinition{
	Name:        "mail_record",
	Module:      "platform",
	Label:       "Mail Record",
	LabelPlural: "Mail Records",
	Description:  "Durable log of every outbound email queued or sent by the platform.",
	DisableAudit: true, // mail records are themselves an audit trail

	Fields: []def.FieldDef{
		{
			Name:      "to_address",
			Type:      def.FieldTypeData,
			Label:     "To",
			Required:  true,
			Immutable: true,
			MaxLen:    254,
		},
		{
			Name:      "from_address",
			Type:      def.FieldTypeData,
			Label:     "From",
			Required:  true,
			Immutable: true,
			MaxLen:    254,
		},
		{
			Name:      "subject",
			Type:      def.FieldTypeData,
			Label:     "Subject",
			Required:  true,
			Immutable: true,
			MaxLen:    998, // RFC 5322 max subject line
		},
		{
			// template_id identifies the mail template used to render the message.
			// Empty for raw/untemplateed messages.
			Name:      "template_id",
			Type:      def.FieldTypeData,
			Label:     "Template",
			Immutable: true,
			MaxLen:    100,
		},
		{
			// body_html is the rendered HTML email body (stored for auditability).
			Name:      "body_html",
			Type:      def.FieldTypeLongText,
			Label:     "HTML Body",
			Immutable: true,
			Hidden:    true, // not shown in list views
		},
		{
			// status tracks the delivery lifecycle.
			Name:    "status",
			Type:    def.FieldTypeSelect,
			Label:   "Status",
			Options: []string{"queued", "sending", "sent", "failed"},
			Default: func() any { return "queued" },
		},
		{
			// sent_at is set when the mail adapter confirms delivery.
			Name:     "sent_at",
			Type:     def.FieldTypeDateTime,
			Label:    "Sent At",
			ReadOnly: true,
		},
		{
			// attempt_count is incremented on each delivery attempt.
			Name:    "attempt_count",
			Type:    def.FieldTypeInt,
			Label:   "Attempts",
			Default: func() any { return 0 },
		},
		{
			// error_message is set on delivery failure; empty on success.
			Name:     "error_message",
			Type:     def.FieldTypeSmallText,
			Label:    "Error",
			ReadOnly: true,
		},
		{
			// provider_message_id is the ID returned by the mail provider (e.g. SES MessageId).
			// Used to correlate delivery events (bounces, opens) from provider webhooks.
			Name:     "provider_message_id",
			Type:     def.FieldTypeData,
			Label:    "Provider Message ID",
			ReadOnly: true,
			MaxLen:   255,
		},
		{
			// context stores optional metadata for tracing: entity_name, entity_id, triggered_by.
			Name:      "context",
			Type:      def.FieldTypeJSON,
			Label:     "Context",
			Immutable: true,
		},
	},

	Permissions: def.PermissionSet{
		// Mail records are created by framework mail service only — no public Create.
		// tenant.admin can inspect mail logs for debugging.
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.mail_record.{read,delete}
		//   role:tenant.admin   → platform.mail_record.read
		Read:   []string{"platform.mail_record.read"},
		Delete: []string{"platform.mail_record.delete"},
	},
}

func init() {
	def.Register(&RecordDefinition)
}

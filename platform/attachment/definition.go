// Package attachment provides the platform_attachment entity definition.
//
// Attachments link uploaded files to any entity record in the system.
// The attachment entity stores file metadata only — the binary content lives
// in an object store (S3, GCS, or local storage). The storage_path field
// holds the object key relative to the configured bucket.
//
// # Access model
//
// Attachment access follows the access rights of the linked entity. If a user
// can read an invoice, they can read that invoice's attachments. Enforcement
// is an application-layer concern (PolicyFunc); RLS enforces tenant isolation.
//
// # File removal
//
// Deleting an attachment record does NOT remove the object from storage.
// A background job (Phase 10) reconciles orphaned objects.
package attachment

import (
	"awo.so/awo/def"
)

// Definition is the platform_attachment entity.
// Each record represents one uploaded file linked to one entity record.
var Definition = def.SystemDefinition{
	Name:        "attachment",
	Module:      "platform",
	Label:       "Attachment",
	LabelPlural: "Attachments",
	Description: "File metadata for an uploaded attachment linked to an entity record.",

	Fields: []def.FieldDef{
		{
			// entity_name identifies the owning entity type (e.g. "finance_invoice").
			Name:      "entity_name",
			Type:      def.FieldTypeData,
			Label:     "Entity",
			Required:  true,
			Immutable: true,
			MaxLen:    100,
		},
		{
			// entity_id is the UUID of the owning record.
			Name:      "entity_id",
			Type:      def.FieldTypeData, // UUID stored as string; no FK (cross-entity)
			Label:     "Entity ID",
			Required:  true,
			Immutable: true,
			MaxLen:    36,
		},
		{
			Name:      "file_name",
			Type:      def.FieldTypeData,
			Label:     "File Name",
			Required:  true,
			Immutable: true,
			MaxLen:    500,
		},
		{
			// content_type is the MIME type (e.g. "application/pdf", "image/png").
			Name:      "content_type",
			Type:      def.FieldTypeData,
			Label:     "Content Type",
			Required:  true,
			Immutable: true,
			MaxLen:    127, // RFC 4288 max type+subtype length
		},
		{
			// storage_path is the object key in the configured storage backend.
			// Format depends on the storage driver; never exposed to end users.
			Name:      "storage_path",
			Type:      def.FieldTypeData,
			Label:     "Storage Path",
			Required:  true,
			Immutable: true,
			Sensitive: true, // path reveals internal storage structure
			MaxLen:    2048,
		},
		{
			// file_size in bytes. Used for display and storage quota enforcement.
			Name:      "file_size",
			Type:      def.FieldTypeInt,
			Label:     "File Size (bytes)",
			Required:  true,
			Immutable: true,
		},
		{
			// checksum is the SHA-256 hex digest of the file content.
			// Used to detect corruption and enable deduplication.
			Name:      "checksum",
			Type:      def.FieldTypeData,
			Label:     "Checksum (SHA-256)",
			Immutable: true,
			MaxLen:    64, // hex-encoded SHA-256 is always 64 chars
		},
		{
			// uploaded_by links to the user who performed the upload.
			// May be nil for programmatic uploads (service accounts).
			Name:       "uploaded_by",
			Type:       def.FieldTypeLink,
			Label:      "Uploaded By",
			LinkTarget: "iam_user",
			Immutable:  true,
		},
	},

	Permissions: def.PermissionSet{
		// Permission identifiers follow "platform.attachment.{operation}".
		// Access to attachments is further restricted by the PolicyFunc to
		// match the access rights of the linked entity record.
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:tenant.user  → platform.attachment.{create,read}
		//   role:tenant.admin → platform.attachment.{create,read,delete}
		Create: []string{"platform.attachment.create"},
		Read:   []string{"platform.attachment.read"},
		Delete: []string{"platform.attachment.delete"},
	},
}

func init() {
	def.Register(&Definition)
}

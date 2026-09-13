package docgen

// Options controls what WriteMarkdown renders. Each field defaults to the
// safer, smaller-output value — see DefaultOptions.
type Options struct {
	// IncludeSensitiveFields includes fields marked Sensitive (e.g. national
	// ID numbers, bank account fields) in output. Default false.
	//
	// This must stay false for any documentation that leaves platform
	// engineering's hands — tenant admins, support staff, or public API
	// consumers should never see sensitive field metadata by default.
	IncludeSensitiveFields bool

	// IncludeHiddenFields includes fields marked Hidden (internal
	// bookkeeping fields not meant for end-user UI, e.g. computed paths
	// or denormalized counters) in output. Default false.
	IncludeHiddenFields bool

	// IncludePermissions includes the RBAC permission matrix per entity
	// (which roles/policies can create/read/write/delete). Default true.
	IncludePermissions bool

	// IncludeRoutes includes the auto-generated REST API route table per
	// entity. Default true.
	IncludeRoutes bool
}

// DefaultOptions returns the options used for public/tenant-facing
// documentation: permissions and routes are shown (they're part of the
// public API contract), but sensitive and hidden fields are not.
func DefaultOptions() Options {
	return Options{
		IncludePermissions: true,
		IncludeRoutes:      true,
	}
}

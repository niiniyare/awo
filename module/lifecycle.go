package module

import "context"

// LifecycleHooks holds optional callbacks that the platform invokes at
// well-defined points in a module's installation lifecycle. Each hook is
// optional — nil fields are silently skipped.
//
// Hooks must be idempotent: the platform may call OnInstall or OnUpgrade more
// than once if the installation workflow is interrupted and retried.
type LifecycleHooks struct {
	// OnInstall is called once when the module is first activated for a tenant.
	// Use it to seed reference data, create default configuration, or register
	// scheduled jobs. Must not run migrations — those are handled by the
	// migration runner before OnInstall is called.
	OnInstall func(ctx context.Context) error

	// OnUpgrade is called when the module version advances for a tenant.
	// from is the previously installed version. The current version is
	// available via the associated Manifest.
	OnUpgrade func(ctx context.Context, from Version) error

	// OnUninstall is called before a module is deactivated for a tenant.
	// Use it to remove scheduled jobs or clean up non-schema artefacts.
	// Schema objects are removed by down-migrations — not here.
	OnUninstall func(ctx context.Context) error
}

// Module pairs a Manifest with its LifecycleHooks. It is the top-level unit
// of registration for business modules that require installation callbacks.
//
// Modules that need no lifecycle hooks can register a bare Manifest via
// [ModuleRegistry.Register].
type Module struct {
	Manifest
	Hooks LifecycleHooks
}

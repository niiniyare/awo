package sdk

import "awo.so/awo/def"

// AdminOnly returns a PermissionSet that restricts all operations to the
// platform-admin and tenant-admin roles.
func AdminOnly() def.PermissionSet {
	roles := []string{"role:platform-admin", "role:tenant.admin"}
	return def.PermissionSet{
		Create: roles,
		Read:   roles,
		Write:  roles,
		Delete: roles,
	}
}

// TenantScoped returns a PermissionSet where tenant.admin manages and
// tenant.user has read access. No platform-admin override needed for
// business entities (they bypass Casbin at the middleware level).
func TenantScoped() def.PermissionSet {
	return def.PermissionSet{
		Create: []string{"role:tenant.admin"},
		Read:   []string{"role:tenant.admin", "role:tenant.user"},
		Write:  []string{"role:tenant.admin"},
		Delete: []string{"role:tenant.admin"},
	}
}

// FullAccess returns a PermissionSet where tenant.user can create, read, and
// write, but only tenant.admin can delete. A common pattern for self-service
// entities (e.g. tasks, comments, contact records).
func FullAccess() def.PermissionSet {
	return def.PermissionSet{
		Create: []string{"role:tenant.admin", "role:tenant.user"},
		Read:   []string{"role:tenant.admin", "role:tenant.user"},
		Write:  []string{"role:tenant.admin", "role:tenant.user"},
		Delete: []string{"role:tenant.admin"},
	}
}

// ReadOnly returns a PermissionSet where records can only be read, not
// created, updated, or deleted through the standard API. Use for entities
// that are written exclusively by framework hooks or background jobs.
func ReadOnly() def.PermissionSet {
	return def.PermissionSet{
		Create: []string{},
		Read:   []string{"role:platform-admin", "role:tenant.admin", "role:tenant.user"},
		Write:  []string{},
		Delete: []string{},
	}
}

// PlatformOnly returns a PermissionSet restricted to platform-admin.
// Use for global infrastructure entities (modules, tenants, flags).
func PlatformOnly() def.PermissionSet {
	return def.PermissionSet{
		Create: []string{"role:platform-admin"},
		Read:   []string{"role:platform-admin"},
		Write:  []string{"role:platform-admin"},
		Delete: []string{"role:platform-admin"},
	}
}

// WithPolicy wraps p with the given policy function. Returns a copy of p
// with Policy set.
func WithPolicy(p def.PermissionSet, policy def.PolicyFunc) def.PermissionSet {
	p.Policy = policy
	return p
}

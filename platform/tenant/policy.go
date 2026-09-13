package tenant

import (
	"context"

	"awo.so/awo/def"
)

// TenantPolicy applies no additional row restriction beyond RLS for tenant
// records. Tenant isolation is enforced at the database level via RLS policies
// and set_tenant_context(). Platform admins see all tenants; tenant admins see
// only their own — this is enforced by Casbin, not row filtering.
var TenantPolicy def.PolicyFunc = func(_ context.Context) def.Filter {
	return nil
}

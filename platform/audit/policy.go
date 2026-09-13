package audit

import (
	"context"

	"awo.so/awo/def"
)

// AuditLogPolicy applies no additional row restriction beyond RBAC.
// Platform admins see all entries; tenant admins see only their tenant's
// entries (enforced at the query layer by passing tenant_id_ref as a filter
// argument, not via PolicyFunc — the audit log has no RLS since it spans
// all tenants).
var AuditLogPolicy def.PolicyFunc = func(_ context.Context) def.Filter {
	return nil
}

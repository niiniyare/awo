package iam

import (
	"context"

	"awo.so/awo/auth"
	"awo.so/awo/def"
	"awo.so/awo/filter"
	"github.com/google/uuid"
)

// userSelfOrAdminPolicy is the row-level filter for iam_user.
//
// Ordinary users see only their own record (filtered by id).
// Viewers with "role:tenant.admin" or "role:platform-admin" see all users
// within their tenant (no additional filter — RLS provides the tenant boundary).
var userSelfOrAdminPolicy def.PolicyFunc = func(ctx context.Context) def.Filter {
	viewer := auth.ViewerFromContext(ctx)
	if viewer.IsPlatformAdmin() || viewer.HasRole("role:tenant.admin") {
		return nil // unrestricted within tenant RLS boundary
	}
	return filter.Eq("id", viewer.UserID())
}

// sessionOwnerPolicy is the row-level filter for iam_session.
//
// Users see only sessions belonging to their own user_id. Tenant.admin sees all
// sessions for all users within the tenant.
var sessionOwnerPolicy def.PolicyFunc = func(ctx context.Context) def.Filter {
	viewer := auth.ViewerFromContext(ctx)
	if viewer.IsPlatformAdmin() || viewer.HasRole("role:tenant.admin") {
		return nil
	}
	return filter.Eq("user_id", viewer.UserID())
}

// loginAuditTenantAdminPolicy restricts login audit reads to tenant administrators
// and platform admins. Ordinary users have no visibility into the audit log.
var loginAuditTenantAdminPolicy def.PolicyFunc = func(ctx context.Context) def.Filter {
	viewer := auth.ViewerFromContext(ctx)
	if viewer.IsPlatformAdmin() || viewer.HasRole("role:tenant.admin") {
		return nil
	}
	// Return a filter that matches nothing — non-admin users see no audit records.
	// The PolicyEvaluator's "iam.login_audit.read" check prevents reaching here
	// for users without the permission; this is a defense-in-depth measure.
	return filter.Eq("id", uuid.Nil) // matches no rows
}

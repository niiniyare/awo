// Package organization implements the Awo platform Organization module.
//
// # Isolation model
//
// Two independent isolation layers:
//
//  1. Tenant isolation (framework) — PostgreSQL RLS, current_tenant_id().
//     The framework guarantees tenant_id filtering and nothing more.
//
//  2. Organization scope (application) — evaluated in Go by this module.
//     OrganizationService.ResolveScope() converts a ViewerContext into a
//     []uuid.UUID that application services pass as an IN predicate.
//     Organization tables carry NO RLS policies.
//
// These layers must never be conflated. ERP modules implement org-aware
// business rules using the primitives in this package without coupling to
// the framework's RLS implementation.
//
// # Entities
//
//   - platform_organization   — org tree node (arbitrary depth, materialized path)
//   - platform_org_type       — tenant-defined type registry (metadata-driven)
//   - platform_org_assignment — user ↔ organization membership with role
//
// # Two-stage authorization
//
// Stage 1 (framework):  tenant isolation via RLS
// Stage 2 (application): org scope via OrganizationService.ResolveScope
//   - operation permissions (RBAC)
//   - business policies (EntityDefinition.Policy)
//
// # Request flow
//
//	HTTP Request
//	  ↓ auth middleware    → session validation, load org assignments
//	ViewerContext          → TenantID, UserID, ActiveOrgID, Assignments, Roles
//	  ↓ service middleware → OrganizationService.ResolveScope()
//	[]uuid.UUID            → visible org IDs
//	  ↓ application service builds filter with org IDs
//	Repository.List()
package organization

import "awo.so/awo/def"

func init() {
	def.Register(&Definition)
	def.Register(&OrgTypeDefinition)
	def.Register(&OrgAssignmentDefinition)
}

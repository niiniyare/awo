package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"

	casbinv2 "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

// CapabilityGrant is an engine-agnostic capability assertion derived from a
// [def.PermissionSet] declaration at compile time. It binds a permission
// identifier to the entity and operation it controls.
//
// CapabilityGrant deliberately contains no reference to roles, users, or any
// specific authorization backend (ADR-011). The [PolicyEvaluator] implementation
// loads CapabilityGrants at startup alongside a separate role-to-permission
// mapping sourced from the IAM module to resolve authorization decisions at
// request time.
//
// CapabilityGrant is defined in this package (not in the compiler) so that the
// auth package is self-contained and extractable as a standalone library without
// pulling in the compiler as a dependency.
type CapabilityGrant struct {
	// Permission is the permission identifier from the PermissionSet.
	// Format: "{module}.{entity}.{operation}"
	// e.g. "finance.invoice.create", "iam.user.read".
	Permission string

	// Entity is the qualified entity name this grant applies to.
	// e.g. "finance_invoice", "iam_user".
	Entity string

	// Action is the operation this grant controls.
	// Standard: "create", "read", "write", "delete".
	// Custom: any action name declared in ActionDef.
	Action string
}

// casbinModel is the Casbin RBAC model text used by the default evaluator.
// It maps permission identifiers (p.sub) to entity+action pairs, and roles
// to permission identifiers via g (role hierarchy / inheritance) assertions.
//
// Authorization flow:
//   - Phase 1 CapabilityGrants: p, {permission}, {entity}, {action}
//     e.g. p, finance.invoice.create, finance_invoice, create
//   - Phase 2 role-to-permission: g, {role}, {permission}
//     e.g. g, role:finance.accounts_payable, finance.invoice.create
//   - Enforcer check: does g(role, permission) && p(permission, entity, action)?
//
// The matcher g(r.sub, p.sub) resolves role-to-permission bindings so that any
// role granted a permission can perform the corresponding entity+action.
const casbinModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

// RolePermission maps a role name to a permission identifier it grants.
// The IAM module loads these from the iam_role_permissions table at startup
// and passes them to [NewCasbinEvaluator].
type RolePermission struct {
	// Role is the role name. Format: "role:{domain}.{name}".
	// e.g. "role:finance.accounts_payable", "role:tenant.admin".
	Role string

	// Permission is the permission identifier granted to this role.
	// Format: "{module}.{entity}.{operation}".
	// e.g. "finance.invoice.create", "finance.invoice.submit".
	Permission string
}

// CasbinEvaluator is the default [PolicyEvaluator] implementation. It uses
// Casbin v2 with a two-phase in-memory policy loading strategy:
//
//   - Phase 1: [CapabilityGrant] values are loaded as Casbin p assertions,
//     binding permission identifiers to entity+action pairs.
//   - Phase 2: [RolePermission] values from the IAM module are loaded as
//     Casbin g assertions, binding role names to permission identifiers.
//
// The enforcer is held in memory and is safe for concurrent reads. Policy
// reload (e.g. after a role-to-permission change) uses a write-locked atomic
// swap via [CasbinEvaluator.Reload] — no restart required.
//
// CasbinEvaluator does NOT handle the platform-admin bypass. That short-circuit
// lives in the authorization middleware before CanPerform is called.
type CasbinEvaluator struct {
	mu       sync.RWMutex
	enforcer *casbinv2.Enforcer
}

// NewCasbinEvaluator constructs a CasbinEvaluator and loads the initial policy
// from grants (Phase 1) and rolePerms (Phase 2).
//
//   - grants comes from [compiler.CompiledSchema.CapabilityGrants], emitted at
//     compile time from each entity's PermissionSet.
//   - rolePerms comes from the IAM module's iam_role_permissions table, loaded
//     at startup via [iam.AuthService.LoadRolePermissions].
//
// Returns an error only if the embedded Casbin model fails to parse (this
// should never happen) or if a policy assertion cannot be added.
func NewCasbinEvaluator(grants []CapabilityGrant, rolePerms []RolePermission) (*CasbinEvaluator, error) {
	enforcer, err := buildEnforcer(grants, rolePerms)
	if err != nil {
		return nil, fmt.Errorf("auth: build casbin enforcer: %w", err)
	}
	return &CasbinEvaluator{enforcer: enforcer}, nil
}

// Ensure CasbinEvaluator implements PolicyEvaluator at compile time.
var _ PolicyEvaluator = (*CasbinEvaluator)(nil)

// CanPerform returns true if any role held by viewer grants permission to
// perform action on object.
//
// The check iterates viewer.Roles() and queries the Casbin enforcer for each
// role. The first matching role short-circuits and returns true. This is O(n)
// in the number of roles, which is very small in practice (< 10).
//
// Platform admins are NOT handled here — the authorization middleware
// short-circuits before calling CanPerform when viewer.IsPlatformAdmin() is true.
func (e *CasbinEvaluator) CanPerform(ctx context.Context, viewer ViewerContext, object string, action string) (bool, error) {
	e.mu.RLock()
	enf := e.enforcer
	e.mu.RUnlock()

	for _, role := range viewer.Roles() {
		ok, err := enf.Enforce(role, object, action)
		if err != nil {
			return false, fmt.Errorf("auth: casbin enforce %q on %s.%s: %w", role, object, action, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// Reload atomically replaces the enforcer with a new one loaded from the
// provided grants and rolePerms. Call this after any role-to-permission change
// in the IAM module to apply the updated policy without restarting the process.
//
// In-flight CanPerform calls using the old enforcer complete before the swap
// takes effect (write-lock ensures no torn reads).
func (e *CasbinEvaluator) Reload(grants []CapabilityGrant, rolePerms []RolePermission) error {
	enforcer, err := buildEnforcer(grants, rolePerms)
	if err != nil {
		return fmt.Errorf("auth: reload casbin enforcer: %w", err)
	}
	e.mu.Lock()
	e.enforcer = enforcer
	e.mu.Unlock()
	return nil
}

// buildEnforcer constructs a Casbin Enforcer from the embedded model and loads
// Phase 1 (CapabilityGrants as p assertions) and Phase 2 (RolePermissions as
// g assertions) policy data programmatically.
func buildEnforcer(grants []CapabilityGrant, rolePerms []RolePermission) (*casbinv2.Enforcer, error) {
	m, err := model.NewModelFromString(strings.TrimSpace(casbinModel))
	if err != nil {
		return nil, fmt.Errorf("parse casbin model: %w", err)
	}

	// Use an empty adapter; all policies are loaded programmatically below.
	// This avoids the need for a file or DB adapter while satisfying the
	// Casbin Enforcer constructor.
	adapter := &memoryAdapter{}
	enforcer, err := casbinv2.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("new casbin enforcer: %w", err)
	}

	// Phase 1: load CapabilityGrants as p assertions.
	// p, {permission}, {entity}, {action}
	// e.g. p, finance.invoice.create, finance_invoice, create
	for _, g := range grants {
		if _, err := enforcer.AddPolicy(g.Permission, g.Entity, g.Action); err != nil {
			return nil, fmt.Errorf("add capability grant p(%s, %s, %s): %w", g.Permission, g.Entity, g.Action, err)
		}
	}

	// Phase 2: load role-to-permission bindings as g assertions.
	// g, {role}, {permission}
	// e.g. g, role:finance.accounts_payable, finance.invoice.create
	for _, rp := range rolePerms {
		if _, err := enforcer.AddRoleForUser(rp.Role, rp.Permission); err != nil {
			return nil, fmt.Errorf("add role permission g(%s, %s): %w", rp.Role, rp.Permission, err)
		}
	}

	return enforcer, nil
}

// memoryAdapter is a no-op Casbin persist.Adapter. All policies are loaded
// programmatically via AddPolicy / AddRoleForUser; this adapter is never used
// for load/save cycles. It is required by the Casbin Enforcer constructor.
type memoryAdapter struct{}

var _ persist.Adapter = (*memoryAdapter)(nil)

func (*memoryAdapter) LoadPolicy(_ model.Model) error                    { return nil }
func (*memoryAdapter) SavePolicy(_ model.Model) error                    { return nil }
func (*memoryAdapter) AddPolicy(_ string, _ string, _ []string) error    { return nil }
func (*memoryAdapter) RemovePolicy(_ string, _ string, _ []string) error { return nil }
func (*memoryAdapter) RemoveFilteredPolicy(_ string, _ string, _ int, _ ...string) error {
	return nil
}

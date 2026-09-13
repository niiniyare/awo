package auth_test

import (
	"context"
	"testing"
	"time"

	"awo.so/awo/auth"
	"awo.so/awo/def"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testViewer is a minimal ViewerContext implementation for tests.
type testViewer struct {
	tenantID         uuid.UUID
	userID           uuid.UUID
	serviceAccountID uuid.UUID
	roles            []string
	roleSet          map[string]bool
}

func newTestViewer(tenantID, userID uuid.UUID, roles ...string) *testViewer {
	set := make(map[string]bool, len(roles))
	for _, r := range roles {
		set[r] = true
	}
	return &testViewer{
		tenantID: tenantID,
		userID:   userID,
		roles:    roles,
		roleSet:  set,
	}
}

func (v *testViewer) TenantID() uuid.UUID         { return v.tenantID }
func (v *testViewer) UserID() uuid.UUID           { return v.userID }
func (v *testViewer) ServiceAccountID() uuid.UUID { return v.serviceAccountID }
func (v *testViewer) Roles() []string             { return v.roles }
func (v *testViewer) HasRole(role string) bool    { return v.roleSet[role] }
func (v *testViewer) IsPlatformAdmin() bool       { return v.roleSet["role:platform-admin"] }
func (v *testViewer) Actor() *def.Actor {
	return &def.Actor{
		UserID:           v.userID,
		ServiceAccountID: v.serviceAccountID,
		TenantID:         v.tenantID,
		Roles:            append([]string(nil), v.roles...),
	}
}

// testGrants returns a minimal set of CapabilityGrants for finance_invoice.
func testGrants() []auth.CapabilityGrant {
	return []auth.CapabilityGrant{
		{Permission: "finance.invoice.create", Entity: "finance_invoice", Action: "create"},
		{Permission: "finance.invoice.read", Entity: "finance_invoice", Action: "read"},
		{Permission: "finance.invoice.update", Entity: "finance_invoice", Action: "write"},
		{Permission: "finance.invoice.delete", Entity: "finance_invoice", Action: "delete"},
		{Permission: "finance.invoice.submit", Entity: "finance_invoice", Action: "submit"},
		{Permission: "finance.invoice.approve", Entity: "finance_invoice", Action: "approve"},
	}
}

// testRolePerms maps roles to the permissions they grant.
func testRolePerms() []auth.RolePermission {
	return []auth.RolePermission{
		{Role: "role:finance.viewer", Permission: "finance.invoice.read"},
		{Role: "role:finance.accounts_payable", Permission: "finance.invoice.create"},
		{Role: "role:finance.accounts_payable", Permission: "finance.invoice.read"},
		{Role: "role:finance.accounts_payable", Permission: "finance.invoice.update"},
		{Role: "role:finance.accounts_payable", Permission: "finance.invoice.submit"},
		{Role: "role:finance.approver", Permission: "finance.invoice.approve"},
		{Role: "role:finance.manager", Permission: "finance.invoice.create"},
		{Role: "role:finance.manager", Permission: "finance.invoice.read"},
		{Role: "role:finance.manager", Permission: "finance.invoice.update"},
		{Role: "role:finance.manager", Permission: "finance.invoice.delete"},
		{Role: "role:finance.manager", Permission: "finance.invoice.submit"},
		{Role: "role:finance.manager", Permission: "finance.invoice.approve"},
		{Role: "role:tenant.admin", Permission: "finance.invoice.create"},
		{Role: "role:tenant.admin", Permission: "finance.invoice.read"},
		{Role: "role:tenant.admin", Permission: "finance.invoice.update"},
		{Role: "role:tenant.admin", Permission: "finance.invoice.delete"},
		{Role: "role:tenant.admin", Permission: "finance.invoice.submit"},
		{Role: "role:tenant.admin", Permission: "finance.invoice.approve"},
	}
}

func TestCasbinEvaluator_CanPerform_Allowed(t *testing.T) {
	eval, err := auth.NewCasbinEvaluator(testGrants(), testRolePerms())
	require.NoError(t, err)

	ctx := context.Background()
	tenantID := uuid.New()

	tests := []struct {
		name   string
		roles  []string
		object string
		action string
	}{
		{
			name:   "viewer can read invoice",
			roles:  []string{"role:finance.viewer"},
			object: "finance_invoice",
			action: "read",
		},
		{
			name:   "accounts_payable can create invoice",
			roles:  []string{"role:finance.accounts_payable"},
			object: "finance_invoice",
			action: "create",
		},
		{
			name:   "accounts_payable can submit invoice",
			roles:  []string{"role:finance.accounts_payable"},
			object: "finance_invoice",
			action: "submit",
		},
		{
			name:   "approver can approve invoice",
			roles:  []string{"role:finance.approver"},
			object: "finance_invoice",
			action: "approve",
		},
		{
			name:   "tenant_admin can delete invoice",
			roles:  []string{"role:tenant.admin"},
			object: "finance_invoice",
			action: "delete",
		},
		{
			name:   "manager can do everything",
			roles:  []string{"role:finance.manager"},
			object: "finance_invoice",
			action: "approve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viewer := newTestViewer(tenantID, uuid.New(), tt.roles...)
			ok, err := eval.CanPerform(ctx, viewer, tt.object, tt.action)
			require.NoError(t, err)
			assert.True(t, ok, "expected allowed")
		})
	}
}

func TestCasbinEvaluator_CanPerform_Denied(t *testing.T) {
	eval, err := auth.NewCasbinEvaluator(testGrants(), testRolePerms())
	require.NoError(t, err)

	ctx := context.Background()
	tenantID := uuid.New()

	tests := []struct {
		name   string
		roles  []string
		object string
		action string
	}{
		{
			name:   "viewer cannot create invoice",
			roles:  []string{"role:finance.viewer"},
			object: "finance_invoice",
			action: "create",
		},
		{
			name:   "accounts_payable cannot approve invoice",
			roles:  []string{"role:finance.accounts_payable"},
			object: "finance_invoice",
			action: "approve",
		},
		{
			name:   "accounts_payable cannot delete invoice",
			roles:  []string{"role:finance.accounts_payable"},
			object: "finance_invoice",
			action: "delete",
		},
		{
			name:   "no roles cannot do anything",
			roles:  []string{},
			object: "finance_invoice",
			action: "read",
		},
		{
			name:   "unknown role denied",
			roles:  []string{"role:unknown"},
			object: "finance_invoice",
			action: "read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viewer := newTestViewer(tenantID, uuid.New(), tt.roles...)
			ok, err := eval.CanPerform(ctx, viewer, tt.object, tt.action)
			require.NoError(t, err)
			assert.False(t, ok, "expected denied")
		})
	}
}

func TestCasbinEvaluator_MultipleRoles_AnyMatchGrants(t *testing.T) {
	eval, err := auth.NewCasbinEvaluator(testGrants(), testRolePerms())
	require.NoError(t, err)

	ctx := context.Background()
	// viewer has viewer + approver — should be able to both read and approve
	viewer := newTestViewer(uuid.New(), uuid.New(),
		"role:finance.viewer",
		"role:finance.approver",
	)

	ok, err := eval.CanPerform(ctx, viewer, "finance_invoice", "read")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = eval.CanPerform(ctx, viewer, "finance_invoice", "approve")
	require.NoError(t, err)
	assert.True(t, ok)

	// but not delete (neither role has it)
	ok, err = eval.CanPerform(ctx, viewer, "finance_invoice", "delete")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCasbinEvaluator_Reload(t *testing.T) {
	eval, err := auth.NewCasbinEvaluator(testGrants(), testRolePerms())
	require.NoError(t, err)

	ctx := context.Background()
	viewer := newTestViewer(uuid.New(), uuid.New(), "role:finance.viewer")

	// Initially viewer can read
	ok, err := eval.CanPerform(ctx, viewer, "finance_invoice", "read")
	require.NoError(t, err)
	assert.True(t, ok)

	// Reload with empty role permissions (revoke all)
	err = eval.Reload(testGrants(), []auth.RolePermission{})
	require.NoError(t, err)

	// After reload, viewer is denied (no role-to-permission mapping)
	ok, err = eval.CanPerform(ctx, viewer, "finance_invoice", "read")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCasbinEvaluator_DenyByDefault_NoPermissionsDeclared(t *testing.T) {
	// Entity with no capability grants = deny all non-platform-admin
	eval, err := auth.NewCasbinEvaluator(
		[]auth.CapabilityGrant{}, // no grants
		[]auth.RolePermission{},  // no role perms
	)
	require.NoError(t, err)

	ctx := context.Background()
	viewer := newTestViewer(uuid.New(), uuid.New(), "role:tenant.admin")

	ok, err := eval.CanPerform(ctx, viewer, "some_entity", "read")
	require.NoError(t, err)
	assert.False(t, ok)
}

// ── Integration: Session → Viewer → CasbinEvaluator ──────────────────────────

func TestFullAuthFlow_SessionToViewer_ToEvaluator(t *testing.T) {
	s := &auth.Session{
		Token:     "test-token",
		UserID:    uuid.New(),
		TenantID:  uuid.New(),
		Roles:     []string{"role:finance.accounts_payable"},
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}

	eval, err := auth.NewCasbinEvaluator(testGrants(), testRolePerms())
	require.NoError(t, err)

	viewer := s.ToViewer()
	ctx := auth.WithViewer(context.Background(), viewer)

	// Retrieve viewer from context and check
	v := auth.ViewerFromContext(ctx)
	assert.True(t, v.HasRole("role:finance.accounts_payable"))

	// Check permissions via evaluator
	ok, err := eval.CanPerform(ctx, v, "finance_invoice", "create")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = eval.CanPerform(ctx, v, "finance_invoice", "approve")
	require.NoError(t, err)
	assert.False(t, ok)
}

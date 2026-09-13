package iam

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/auth"
	"awo.so/awo/filter"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func ctxWithViewer(v auth.ViewerContext) context.Context {
	return auth.WithViewer(context.Background(), v)
}

func nowPlusHour() time.Time    { return time.Now().Add(time.Hour) }
func nowMinusMinute() time.Time { return time.Now().Add(-time.Minute) }

func sessionViewer(userID uuid.UUID, roles ...string) auth.ViewerContext {
	s := &auth.Session{
		Token:     "test",
		UserID:    userID,
		TenantID:  uuid.New(),
		Roles:     roles,
		ExpiresAt: nowPlusHour(),
		IssuedAt:  nowMinusMinute(),
	}
	return s.ToViewer()
}

// asEqFilter casts a policy filter result to *filter.Filter and asserts it is a KindEq node.
func asEqFilter(t *testing.T, f any) *filter.Filter {
	t.Helper()
	require.NotNil(t, f)
	ff, ok := f.(*filter.Filter)
	require.True(t, ok, "expected *filter.Filter, got %T", f)
	assert.Equal(t, filter.KindEq, ff.Kind)
	return ff
}

// ── userSelfOrAdminPolicy ─────────────────────────────────────────────────────

func TestUserSelfOrAdminPolicy_OrdinaryUser_FiltersBySelf(t *testing.T) {
	userID := uuid.New()
	ctx := ctxWithViewer(sessionViewer(userID, "role:finance.viewer"))

	f := userSelfOrAdminPolicy(ctx)
	ff := asEqFilter(t, f)
	assert.Equal(t, "id", ff.Field)
	assert.Equal(t, userID, ff.Value)
}

func TestUserSelfOrAdminPolicy_TenantAdmin_ReturnsNil(t *testing.T) {
	ctx := ctxWithViewer(sessionViewer(uuid.New(), "role:tenant.admin"))
	assert.Nil(t, userSelfOrAdminPolicy(ctx))
}

func TestUserSelfOrAdminPolicy_PlatformAdmin_ReturnsNil(t *testing.T) {
	ctx := ctxWithViewer(auth.NewSystemViewer(uuid.New()))
	assert.Nil(t, userSelfOrAdminPolicy(ctx))
}

// ── sessionOwnerPolicy ────────────────────────────────────────────────────────

func TestSessionOwnerPolicy_OrdinaryUser_FiltersByUserID(t *testing.T) {
	userID := uuid.New()
	ctx := ctxWithViewer(sessionViewer(userID, "role:finance.viewer"))

	f := sessionOwnerPolicy(ctx)
	ff := asEqFilter(t, f)
	assert.Equal(t, "user_id", ff.Field)
	assert.Equal(t, userID, ff.Value)
}

func TestSessionOwnerPolicy_TenantAdmin_ReturnsNil(t *testing.T) {
	ctx := ctxWithViewer(sessionViewer(uuid.New(), "role:tenant.admin"))
	assert.Nil(t, sessionOwnerPolicy(ctx))
}

// ── loginAuditTenantAdminPolicy ───────────────────────────────────────────────

func TestLoginAuditTenantAdminPolicy_OrdinaryUser_DenyAllFilter(t *testing.T) {
	ctx := ctxWithViewer(sessionViewer(uuid.New(), "role:finance.viewer"))

	f := loginAuditTenantAdminPolicy(ctx)
	ff := asEqFilter(t, f)
	// Must restrict to id = uuid.Nil, which matches no rows.
	assert.Equal(t, "id", ff.Field)
	assert.Equal(t, uuid.Nil, ff.Value)
}

func TestLoginAuditTenantAdminPolicy_TenantAdmin_ReturnsNil(t *testing.T) {
	ctx := ctxWithViewer(sessionViewer(uuid.New(), "role:tenant.admin"))
	assert.Nil(t, loginAuditTenantAdminPolicy(ctx))
}

func TestLoginAuditTenantAdminPolicy_PlatformAdmin_ReturnsNil(t *testing.T) {
	ctx := ctxWithViewer(auth.NewSystemViewer(uuid.New()))
	assert.Nil(t, loginAuditTenantAdminPolicy(ctx))
}

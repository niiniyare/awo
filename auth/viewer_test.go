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

// ── ViewerContext / DefaultViewer ─────────────────────────────────────────────

func TestNewViewer_FromSession(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	s := &auth.Session{
		Token:     "tok",
		UserID:    userID,
		TenantID:  tenantID,
		Roles:     []string{"role:tenant.admin", "role:finance.viewer"},
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	v := auth.NewViewer(s)

	assert.Equal(t, tenantID, v.TenantID())
	assert.Equal(t, userID, v.UserID())
	assert.Equal(t, uuid.Nil, v.ServiceAccountID())
	assert.Equal(t, []string{"role:tenant.admin", "role:finance.viewer"}, v.Roles())
	assert.True(t, v.HasRole("role:tenant.admin"))
	assert.True(t, v.HasRole("role:finance.viewer"))
	assert.False(t, v.HasRole("role:finance.manager"))
	assert.False(t, v.IsPlatformAdmin())
}

func TestDefaultViewer_IsPlatformAdmin(t *testing.T) {
	s := &auth.Session{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		Roles:     []string{"role:platform-admin"},
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	v := auth.NewViewer(s)
	assert.True(t, v.IsPlatformAdmin())
}

func TestWithViewer_ViewerFromContext(t *testing.T) {
	s := &auth.Session{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		Roles:     []string{"role:tenant.user"},
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	v := auth.NewViewer(s)
	ctx := auth.WithViewer(context.Background(), v)

	got := auth.ViewerFromContext(ctx)
	assert.Equal(t, v.TenantID(), got.TenantID())
	assert.Equal(t, v.UserID(), got.UserID())
}

func TestViewerFromContext_PanicsWhenAbsent(t *testing.T) {
	assert.Panics(t, func() {
		auth.ViewerFromContext(context.Background())
	}, "expected panic when ViewerContext is absent from context")
}

// ── SystemViewer ──────────────────────────────────────────────────────────────

func TestSystemViewer(t *testing.T) {
	tenantID := uuid.New()
	v := auth.NewSystemViewer(tenantID)

	assert.Equal(t, tenantID, v.TenantID())
	assert.Equal(t, uuid.Nil, v.UserID())
	assert.Equal(t, uuid.Nil, v.ServiceAccountID())
	assert.True(t, v.IsPlatformAdmin())
	assert.True(t, v.HasRole("role:platform-admin"))
	assert.False(t, v.HasRole("role:tenant.admin"))
	assert.Equal(t, []string{"role:platform-admin"}, v.Roles())
}

// ── Session ───────────────────────────────────────────────────────────────────

func TestSession_IsExpired(t *testing.T) {
	s := &auth.Session{ExpiresAt: time.Now().Add(-time.Second)}
	assert.True(t, s.IsExpired(time.Now()))

	s2 := &auth.Session{ExpiresAt: time.Now().Add(time.Hour)}
	assert.False(t, s2.IsExpired(time.Now()))
}

func TestSession_ToActor(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	s := &auth.Session{
		UserID:    userID,
		TenantID:  tenantID,
		Roles:     []string{"role:tenant.admin"},
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	actor := s.ToActor()
	require.NotNil(t, actor)
	assert.Equal(t, userID, actor.UserID)
	assert.Equal(t, tenantID, actor.TenantID)
	assert.Equal(t, []string{"role:tenant.admin"}, actor.Roles)
	assert.False(t, actor.IsServiceAccount())
}

func TestSession_ToViewer(t *testing.T) {
	s := &auth.Session{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		Roles:     []string{"role:finance.viewer"},
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	v := s.ToViewer()
	require.NotNil(t, v)
	assert.True(t, v.HasRole("role:finance.viewer"))
}


func TestGenerateToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, err := auth.GenerateToken()
		require.NoError(t, err)
		require.False(t, tokens[tok], "token collision at iteration %d", i)
		tokens[tok] = true
		assert.GreaterOrEqual(t, len(tok), 43, "base64url 256 bits = 43 chars min")
	}
}

// ── Actor (def.Actor) ─────────────────────────────────────────────────────────

func TestActor_IsPlatformAdmin(t *testing.T) {
	a := &def.Actor{Roles: []string{"role:platform-admin"}}
	assert.True(t, a.IsPlatformAdmin())

	b := &def.Actor{Roles: []string{"role:tenant.admin"}}
	assert.False(t, b.IsPlatformAdmin())
}

func TestActor_IsServiceAccount(t *testing.T) {
	a := &def.Actor{ServiceAccountID: uuid.New()}
	assert.True(t, a.IsServiceAccount())

	b := &def.Actor{UserID: uuid.New()}
	assert.False(t, b.IsServiceAccount())
}

func TestActor_HasRole(t *testing.T) {
	a := &def.Actor{Roles: []string{"role:tenant.admin", "role:finance.viewer"}}
	assert.True(t, a.HasRole("role:tenant.admin"))
	assert.True(t, a.HasRole("role:finance.viewer"))
	assert.False(t, a.HasRole("role:finance.manager"))
}

package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	. "awo.so/awo/auth"
)

// ── Session ───────────────────────────────────────────────────────────────────

func TestSession_IsExpired_False_BeforeExpiry(t *testing.T) {
	s := &Session{ExpiresAt: time.Now().Add(time.Hour)}
	if s.IsExpired(time.Now()) {
		t.Fatal("session should not be expired before ExpiresAt")
	}
}

func TestSession_IsExpired_True_AfterExpiry(t *testing.T) {
	s := &Session{ExpiresAt: time.Now().Add(-time.Second)}
	if !s.IsExpired(time.Now()) {
		t.Fatal("session should be expired after ExpiresAt")
	}
}

func TestSession_TTL_Positive_BeforeExpiry(t *testing.T) {
	now := time.Now()
	s := &Session{ExpiresAt: now.Add(30 * time.Minute)}
	ttl := s.TTL(now)
	if ttl <= 0 {
		t.Fatalf("expected positive TTL, got %v", ttl)
	}
}

func TestSession_TTL_Negative_AfterExpiry(t *testing.T) {
	now := time.Now()
	s := &Session{ExpiresAt: now.Add(-time.Minute)}
	ttl := s.TTL(now)
	if ttl >= 0 {
		t.Fatalf("expected negative TTL for expired session, got %v", ttl)
	}
}

func TestSession_ToActor_CopiesRoles(t *testing.T) {
	roles := []string{"role:tenant.admin", "role:finance.read"}
	s := &Session{
		UserID:   uuid.New(),
		TenantID: uuid.New(),
		Roles:    roles,
	}
	actor := s.ToActor()
	if actor == nil {
		t.Fatal("ToActor must not return nil")
	}
	if len(actor.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(actor.Roles))
	}
	// Mutating original must not affect actor.
	s.Roles[0] = "role:mutated"
	if actor.Roles[0] == "role:mutated" {
		t.Error("ToActor must return a defensive copy of roles")
	}
}

func TestSession_ToViewer_ReturnsViewerContext(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	s := &Session{
		TenantID: tenantID,
		UserID:   userID,
		Roles:    []string{"role:tenant.admin"},
	}
	v := s.ToViewer()
	if v == nil {
		t.Fatal("ToViewer must not return nil")
	}
	if v.TenantID() != tenantID {
		t.Errorf("TenantID mismatch")
	}
	if v.UserID() != userID {
		t.Errorf("UserID mismatch")
	}
}

func TestSession_Metadata_RoundTrip(t *testing.T) {
	s := &Session{
		Metadata: map[string]any{
			"finance:last_ap_view": "2026-08-18",
			"awo:mfa_verified":     true,
		},
	}
	if s.Metadata["finance:last_ap_view"] != "2026-08-18" {
		t.Errorf("metadata round-trip failed")
	}
	if s.Metadata["awo:mfa_verified"] != true {
		t.Errorf("metadata bool round-trip failed")
	}
}

func TestGenerateToken_UniqueAndNonEmpty(t *testing.T) {
	t1, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	t2, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if t1 == "" {
		t.Fatal("token must not be empty")
	}
	if t1 == t2 {
		t.Fatal("two GenerateToken calls must return different tokens")
	}
}

func TestGenerateToken_Entropy(t *testing.T) {
	tok, _ := GenerateToken()
	// base64url without padding: 32 bytes → 43 chars
	if len(tok) < 40 {
		t.Errorf("token too short (%d chars), expected ≥40", len(tok))
	}
}

// ── DefaultViewer / NewViewer ─────────────────────────────────────────────────

func TestNewViewer_TenantID(t *testing.T) {
	tenantID := uuid.New()
	s := &Session{TenantID: tenantID, Roles: []string{}}
	v := NewViewer(s)
	if v.TenantID() != tenantID {
		t.Errorf("expected TenantID %v, got %v", tenantID, v.TenantID())
	}
}

func TestNewViewer_HasRole_True(t *testing.T) {
	s := &Session{Roles: []string{"role:tenant.admin", "role:finance.read"}}
	v := NewViewer(s)
	if !v.HasRole("role:tenant.admin") {
		t.Error("expected HasRole('role:tenant.admin') = true")
	}
}

func TestNewViewer_HasRole_False(t *testing.T) {
	s := &Session{Roles: []string{"role:tenant.admin"}}
	v := NewViewer(s)
	if v.HasRole("role:platform-admin") {
		t.Error("expected HasRole('role:platform-admin') = false for non-admin")
	}
}

func TestNewViewer_IsPlatformAdmin_True(t *testing.T) {
	s := &Session{Roles: []string{"role:platform-admin"}}
	v := NewViewer(s)
	if !v.IsPlatformAdmin() {
		t.Error("expected IsPlatformAdmin() = true for 'role:platform-admin'")
	}
}

func TestNewViewer_IsPlatformAdmin_False(t *testing.T) {
	s := &Session{Roles: []string{"role:tenant.admin"}}
	v := NewViewer(s)
	if v.IsPlatformAdmin() {
		t.Error("expected IsPlatformAdmin() = false for tenant admin")
	}
}

func TestNewViewer_Roles_DefensiveCopy(t *testing.T) {
	original := []string{"role:tenant.admin"}
	s := &Session{Roles: original}
	v := NewViewer(s)
	// Mutate original — viewer must not see the change.
	original[0] = "role:mutated"
	if v.Roles()[0] == "role:mutated" {
		t.Error("NewViewer must store a defensive copy of roles")
	}
}

func TestNewViewer_Actor_PropagatesTenantID(t *testing.T) {
	tenantID := uuid.New()
	s := &Session{TenantID: tenantID, Roles: []string{}}
	v := NewViewer(s)
	actor := v.Actor()
	if actor.TenantID != tenantID {
		t.Errorf("Actor.TenantID mismatch")
	}
}

func TestNewViewer_ServiceAccount_Session(t *testing.T) {
	saID := uuid.New()
	s := &Session{
		UserID:           uuid.Nil,
		ServiceAccountID: saID,
		Roles:            []string{"role:svc.importer"},
	}
	v := NewViewer(s)
	if v.UserID() != uuid.Nil {
		t.Errorf("expected UserID=Nil for service account session")
	}
	if v.ServiceAccountID() != saID {
		t.Errorf("ServiceAccountID mismatch")
	}
}

// ── WithViewer / ViewerFromContext ─────────────────────────────────────────────

func TestWithViewer_ViewerFromContext_RoundTrip(t *testing.T) {
	tenantID := uuid.New()
	s := &Session{TenantID: tenantID, Roles: []string{}}
	v := NewViewer(s)

	ctx := WithViewer(context.Background(), v)
	got := ViewerFromContext(ctx)
	if got.TenantID() != tenantID {
		t.Errorf("ViewerFromContext TenantID mismatch")
	}
}

func TestViewerFromContext_Panics_WhenAbsent(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when ViewerContext absent from context")
		}
	}()
	ViewerFromContext(context.Background())
}

// ── SessionValidator (mock impl) ──────────────────────────────────────────────

// mockValidator is a test-local SessionValidator implementation.
// It resolves tokens from a pre-populated map; absent tokens return ErrSessionNotFound.
type mockValidator struct {
	sessions map[string]*Session
	apiSessions map[string]*Session
	infraErr error
}

func (m *mockValidator) ValidateToken(_ context.Context, token string) (*Session, error) {
	if m.infraErr != nil {
		return nil, m.infraErr
	}
	s, ok := m.sessions[token]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return s, nil
}

func (m *mockValidator) ValidateAPIToken(_ context.Context, rawToken string) (*Session, error) {
	if m.infraErr != nil {
		return nil, m.infraErr
	}
	s, ok := m.apiSessions[rawToken]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return s, nil
}

// Compile-time interface check.
var _ SessionValidator = (*mockValidator)(nil)

func TestSessionValidator_ValidToken_ReturnsSession(t *testing.T) {
	tenantID := uuid.New()
	tok := "valid-token-abc"
	v := &mockValidator{
		sessions: map[string]*Session{
			tok: {Token: tok, TenantID: tenantID, Roles: []string{"role:tenant.admin"}},
		},
	}
	s, err := v.ValidateToken(context.Background(), tok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TenantID != tenantID {
		t.Errorf("TenantID mismatch: got %v", s.TenantID)
	}
}

func TestSessionValidator_UnknownToken_ReturnsErrSessionNotFound(t *testing.T) {
	v := &mockValidator{sessions: map[string]*Session{}}
	_, err := v.ValidateToken(context.Background(), "ghost-token")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionValidator_ExpiredSession_ReturnsErrSessionNotFound(t *testing.T) {
	// Expired sessions should not appear in the store (store enforces TTL).
	// Mock: absent key simulates TTL eviction.
	v := &mockValidator{sessions: map[string]*Session{}}
	_, err := v.ValidateToken(context.Background(), "expired-token")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound for expired session, got: %v", err)
	}
}

func TestSessionValidator_InfraError_NotErrSessionNotFound(t *testing.T) {
	infraErr := errors.New("redis: connection refused")
	v := &mockValidator{infraErr: infraErr}
	_, err := v.ValidateToken(context.Background(), "any-token")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrSessionNotFound) {
		t.Error("infra error must NOT be ErrSessionNotFound — callers map it to 503 not 401")
	}
}

func TestSessionValidator_APIToken_ValidReturnsServiceAccountSession(t *testing.T) {
	saID := uuid.New()
	rawTok := "svc-api-token-xyz"
	v := &mockValidator{
		apiSessions: map[string]*Session{
			rawTok: {ServiceAccountID: saID, UserID: uuid.Nil, Roles: []string{"role:svc.importer"}},
		},
	}
	s, err := v.ValidateAPIToken(context.Background(), rawTok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ServiceAccountID != saID {
		t.Errorf("ServiceAccountID mismatch")
	}
	if s.UserID != uuid.Nil {
		t.Errorf("UserID must be Nil for service account session")
	}
}

func TestSessionValidator_APIToken_UnknownReturnsErrSessionNotFound(t *testing.T) {
	v := &mockValidator{apiSessions: map[string]*Session{}}
	_, err := v.ValidateAPIToken(context.Background(), "unknown-api-token")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestErrSessionNotFound_Sentinel_NotNil(t *testing.T) {
	if ErrSessionNotFound == nil {
		t.Fatal("ErrSessionNotFound must not be nil")
	}
}

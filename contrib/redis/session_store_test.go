package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"awo.so/awo/auth"
	contribredis "awo.so/awo/contrib/redis"
)

// newTestStore creates a RedisSessionStore backed by an in-memory miniredis instance.
// miniredis is cleaned up automatically via t.Cleanup.
func newTestStore(t *testing.T) (*contribredis.RedisSessionStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return contribredis.NewSessionStore(rdb), mr
}

// newUserSession creates a valid human-user session with the given TTL.
func newUserSession(tenantID, userID uuid.UUID, ttl time.Duration) *auth.Session {
	tok, _ := auth.GenerateToken()
	now := time.Now()
	return &auth.Session{
		Token:            tok,
		TenantID:         tenantID,
		UserID:           userID,
		ServiceAccountID: uuid.Nil,
		Roles:            []string{"role:tenant.admin"},
		IssuedAt:         now,
		ExpiresAt:        now.Add(ttl),
	}
}

// newServiceAccountSession creates a valid service-account session.
func newServiceAccountSession(tenantID, saID uuid.UUID, ttl time.Duration) *auth.Session {
	tok, _ := auth.GenerateToken()
	now := time.Now()
	return &auth.Session{
		Token:            tok,
		TenantID:         tenantID,
		UserID:           uuid.Nil,
		ServiceAccountID: saID,
		Roles:            []string{"role:svc.importer"},
		IssuedAt:         now,
		ExpiresAt:        now.Add(ttl),
	}
}

// ── Store / Load ──────────────────────────────────────────────────────────────

func TestRedisSessionStore_StoreAndLoad_RoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	tenantID := uuid.New()
	userID := uuid.New()
	s := newUserSession(tenantID, userID, time.Hour)

	if err := store.Store(context.Background(), s); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := store.Load(context.Background(), s.Token)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != s.Token {
		t.Errorf("Token mismatch: got %q, want %q", got.Token, s.Token)
	}
	if got.TenantID != tenantID {
		t.Errorf("TenantID mismatch: got %v, want %v", got.TenantID, tenantID)
	}
	if got.UserID != userID {
		t.Errorf("UserID mismatch")
	}
}

func TestRedisSessionStore_Load_UnknownToken_ReturnsErrSessionNotFound(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.Load(context.Background(), "ghost-token")
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestRedisSessionStore_ExpiredSession_ReturnsErrSessionNotFound(t *testing.T) {
	store, mr := newTestStore(t)
	s := newUserSession(uuid.New(), uuid.New(), time.Second)

	if err := store.Store(context.Background(), s); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// Fast-forward miniredis TTL.
	mr.FastForward(2 * time.Second)

	_, err := store.Load(context.Background(), s.Token)
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound for expired session, got %v", err)
	}
}

func TestRedisSessionStore_Store_AlreadyExpired_ReturnsError(t *testing.T) {
	store, _ := newTestStore(t)
	s := newUserSession(uuid.New(), uuid.New(), -time.Second) // already expired
	err := store.Store(context.Background(), s)
	if err == nil {
		t.Fatal("expected error when storing already-expired session")
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestRedisSessionStore_Delete_RemovesSession(t *testing.T) {
	store, _ := newTestStore(t)
	s := newUserSession(uuid.New(), uuid.New(), time.Hour)
	_ = store.Store(context.Background(), s)

	if err := store.Delete(context.Background(), s); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := store.Load(context.Background(), s.Token)
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound after Delete, got %v", err)
	}
}

// ── ListUserTokens / DeleteAll ────────────────────────────────────────────────

func TestRedisSessionStore_ListUserTokens_ReturnsBothTokens(t *testing.T) {
	store, _ := newTestStore(t)
	tenantID := uuid.New()
	userID := uuid.New()

	s1 := newUserSession(tenantID, userID, time.Hour)
	s2 := newUserSession(tenantID, userID, time.Hour)
	_ = store.Store(context.Background(), s1)
	_ = store.Store(context.Background(), s2)

	tokens, err := store.ListUserTokens(context.Background(), tenantID, userID)
	if err != nil {
		t.Fatalf("ListUserTokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}


func TestRedisSessionStore_DeleteAll_RemovesAllSessions(t *testing.T) {
	store, _ := newTestStore(t)
	tenantID := uuid.New()
	userID := uuid.New()

	s1 := newUserSession(tenantID, userID, time.Hour)
	s2 := newUserSession(tenantID, userID, time.Hour)
	_ = store.Store(context.Background(), s1)
	_ = store.Store(context.Background(), s2)

	if err := store.DeleteAll(context.Background(), tenantID, userID, []string{s1.Token, s2.Token}); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	_, e1 := store.Load(context.Background(), s1.Token)
	_, e2 := store.Load(context.Background(), s2.Token)
	if !errors.Is(e1, auth.ErrSessionNotFound) || !errors.Is(e2, auth.ErrSessionNotFound) {
		t.Error("expected both sessions removed after DeleteAll")
	}

	// Index cleared too.
	tokens, _ := store.ListUserTokens(context.Background(), tenantID, userID)
	if len(tokens) != 0 {
		t.Errorf("expected empty index after DeleteAll, got %d entries", len(tokens))
	}
}

// ── BUG-001: service account session index ────────────────────────────────────

func TestRedisSessionStore_ServiceAccount_NotIndexedUnderNilUserID(t *testing.T) {
	// BUG-001 fix: service account sessions must NOT be written to the
	// user_sessions:{tenantID}:{userID} index. Before the fix, all service
	// accounts shared one index key under uuid.Nil, corrupting bulk revocation.
	store, _ := newTestStore(t)
	tenantID := uuid.New()
	saID := uuid.New()

	s := newServiceAccountSession(tenantID, saID, time.Hour)
	if err := store.Store(context.Background(), s); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// The user_sessions index for uuid.Nil must be empty.
	tokens, err := store.ListUserTokens(context.Background(), tenantID, uuid.Nil)
	if err != nil {
		t.Fatalf("ListUserTokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("BUG-001: service account session indexed under uuid.Nil; got %d tokens, want 0", len(tokens))
	}
}

func TestRedisSessionStore_ServiceAccount_SessionStillLoadable(t *testing.T) {
	// Service account sessions are not indexed but must still be loadable by token.
	store, _ := newTestStore(t)
	saID := uuid.New()
	s := newServiceAccountSession(uuid.New(), saID, time.Hour)

	if err := store.Store(context.Background(), s); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := store.Load(context.Background(), s.Token)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ServiceAccountID != saID {
		t.Errorf("ServiceAccountID mismatch")
	}
}

// ── Interface compliance ──────────────────────────────────────────────────────

func TestRedisSessionStore_ImplementsSessionStore(t *testing.T) {
	store, _ := newTestStore(t)
	var _ auth.SessionStore = store
}

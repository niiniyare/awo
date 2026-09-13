package iam_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	contribredis "awo.so/awo/contrib/redis"
	"awo.so/awo/def"
	"awo.so/awo/platform/iam"
	"awo.so/awo/runtime"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newUserRecord(fields map[string]any) *def.EntityRecord {
	data := make(map[string]any, len(fields))
	for k, v := range fields {
		data[k] = v
	}
	return &def.EntityRecord{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Data:     data,
	}
}

// ── UserPasswordHasher ────────────────────────────────────────────────────────

func TestUserPasswordHasher_BeforeCreate_HashesPassword(t *testing.T) {
	h := &iam.UserPasswordHasher{}
	rec := newUserRecord(map[string]any{"password_hash": "my-secret-password"})

	err := h.BeforeCreate(context.Background(), rec)
	require.NoError(t, err)

	stored := rec.GetString("password_hash")
	assert.NotEqual(t, "my-secret-password", stored, "plain text must be replaced")
	// Must be a valid bcrypt hash.
	err = bcrypt.CompareHashAndPassword([]byte(stored), []byte("my-secret-password"))
	assert.NoError(t, err, "stored value must be a bcrypt hash of the original password")
}

func TestUserPasswordHasher_BeforeCreate_EmptyPassword_ReturnsValidationError(t *testing.T) {
	h := &iam.UserPasswordHasher{}
	rec := newUserRecord(map[string]any{"password_hash": ""})

	err := h.BeforeCreate(context.Background(), rec)
	require.Error(t, err)

	var ve *runtime.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Fields, "password_hash")
}

func TestUserPasswordHasher_BeforeCreate_ShortPassword_ReturnsValidationError(t *testing.T) {
	h := &iam.UserPasswordHasher{}
	rec := newUserRecord(map[string]any{"password_hash": "short"})

	err := h.BeforeCreate(context.Background(), rec)
	require.Error(t, err)

	var ve *runtime.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Fields, "password_hash")
}

func TestUserPasswordHasher_BeforeUpdate_EmptyPassword_RestoresPrevHash(t *testing.T) {
	h := &iam.UserPasswordHasher{}
	prev := newUserRecord(map[string]any{"password_hash": "$2a$12$existinghashvalue"})
	rec := newUserRecord(map[string]any{"password_hash": ""})

	err := h.BeforeUpdate(context.Background(), rec, prev)
	require.NoError(t, err)
	// Plain text was empty → existing hash is preserved.
	assert.Equal(t, "$2a$12$existinghashvalue", rec.GetString("password_hash"))
}

func TestUserPasswordHasher_BeforeUpdate_NewPassword_ReplacesHash(t *testing.T) {
	h := &iam.UserPasswordHasher{}
	prev := newUserRecord(map[string]any{"password_hash": "$2a$12$oldhash"})
	rec := newUserRecord(map[string]any{"password_hash": "new-password-123"})

	err := h.BeforeUpdate(context.Background(), rec, prev)
	require.NoError(t, err)

	stored := rec.GetString("password_hash")
	assert.NotEqual(t, "new-password-123", stored)
	err = bcrypt.CompareHashAndPassword([]byte(stored), []byte("new-password-123"))
	assert.NoError(t, err)
}

// ── UserRoleChangeHook ────────────────────────────────────────────────────────

func setupRoleHookRedis(t *testing.T, tenantID, userID uuid.UUID, tokens []string) (*miniredis.Miniredis, *goredis.Client) {
	t.Helper()
	mr, client := newTestRedis(t)

	// Populate the user_sessions index sorted set.
	// Scores must be future Unix timestamps so ListUserTokens does not prune
	// them via ZREMRANGEBYSCORE before returning the token list.
	indexKey := fmt.Sprintf("user_sessions:%s:%s", tenantID, userID)
	futureBase := float64(time.Now().Add(time.Hour).Unix())
	for i, tok := range tokens {
		client.ZAdd(context.Background(), indexKey, &goredis.Z{Score: futureBase + float64(i), Member: tok})
		client.Set(context.Background(), "session:"+tok, `{"token":"`+tok+`"}`, 0)
	}
	return mr, client
}

func TestUserRoleChangeHook_AfterCreate_RevokesUserSessions(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	tokens := []string{"tok1", "tok2", "tok3"}

	_, client := setupRoleHookRedis(t, tenantID, userID, tokens)
	h := &iam.UserRoleChangeHook{Sessions: contribredis.NewSessionStore(client)}

	rec := &def.EntityRecord{
		ID:       uuid.New(),
		TenantID: tenantID,
		Data:     map[string]any{"user_id": userID},
	}

	err := h.AfterCreate(context.Background(), rec)
	require.NoError(t, err)

	ctx := context.Background()
	// All session keys must be deleted.
	for _, tok := range tokens {
		exists, _ := client.Exists(ctx, "session:"+tok).Result()
		assert.Equal(t, int64(0), exists, "session:%s should be deleted", tok)
	}
	// Index set must also be deleted.
	indexKey := fmt.Sprintf("user_sessions:%s:%s", tenantID, userID)
	exists, _ := client.Exists(ctx, indexKey).Result()
	assert.Equal(t, int64(0), exists, "session index should be deleted")
}

func TestUserRoleChangeHook_NoSessions_IsNoop(t *testing.T) {
	_, client := newTestRedis(t)
	h := &iam.UserRoleChangeHook{Sessions: contribredis.NewSessionStore(client)}

	rec := &def.EntityRecord{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Data:     map[string]any{"user_id": uuid.New()},
	}

	// No sessions in Redis → hook must succeed silently.
	err := h.AfterDelete(context.Background(), rec)
	assert.NoError(t, err)
}

// ── LoginAuditImmutableGuard ──────────────────────────────────────────────────

func TestLoginAuditImmutableGuard_BeforeUpdate_Returns403(t *testing.T) {
	g := &iam.LoginAuditImmutableGuard{}
	err := g.BeforeUpdate(context.Background(), newUserRecord(nil), newUserRecord(nil))
	require.Error(t, err)

	var be *runtime.BusinessError
	require.ErrorAs(t, err, &be)
	assert.Equal(t, 403, be.Status)
	assert.Equal(t, "iam.login_audit.immutable", be.Code)
}

func TestLoginAuditImmutableGuard_BeforeDelete_Returns403(t *testing.T) {
	g := &iam.LoginAuditImmutableGuard{}
	err := g.BeforeDelete(context.Background(), newUserRecord(nil))
	require.Error(t, err)

	var be *runtime.BusinessError
	require.ErrorAs(t, err, &be)
	assert.Equal(t, 403, be.Status)
	assert.Equal(t, "iam.login_audit.immutable", be.Code)
}

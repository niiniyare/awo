// Package redis — session_store.go
//
// RedisSessionStore implements [auth.SessionStore] using go-redis sorted sets
// for the per-user session index and plain string keys for session payloads.
//
// # Key layout
//
//	session:{token}                      — JSON-encoded auth.Session; TTL = ExpiresAt
//	user_sessions:{tenantID}:{userID}    — sorted set; member = token, score = expiry unix
//
// The sorted set enables O(log N) pruning of expired members via ZREMRANGEBYSCORE
// and O(N) enumeration for bulk revocation. This is the only data structure that
// supports expiry-aware enumeration without a full key scan.
//
// # Error handling
//
// Session key writes (SET, DEL on primary key) are critical path — errors are
// returned to callers. Index writes (ZADD, ZREM, ZREMRANGEBYSCORE) are
// best-effort — errors are logged via slog but do not cause the method to fail.
// This asymmetry is intentional: a missing index entry impairs bulk revocation
// but does not compromise session validity or security.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"awo.so/awo/auth"
)

// RedisSessionStore implements [auth.SessionStore] backed by Redis.
// Construct with [NewSessionStore].
type RedisSessionStore struct {
	rdb *goredis.Client
}

// NewSessionStore creates a RedisSessionStore from an existing go-redis client.
func NewSessionStore(rdb *goredis.Client) *RedisSessionStore {
	return &RedisSessionStore{rdb: rdb}
}

// Ensure compile-time interface satisfaction.
var _ auth.SessionStore = (*RedisSessionStore)(nil)

// sessionKey returns the Redis key for a session payload.
// Format: "session:{token}"
func sessionKey(token string) string {
	return "session:" + token
}

// userIndexKey returns the Redis sorted-set key for a user's session index.
// Format: "user_sessions:{tenantID}:{userID}"
func userIndexKey(tenantID, userID uuid.UUID) string {
	return fmt.Sprintf("user_sessions:%s:%s", tenantID, userID)
}

// Store persists session as a JSON string under session:{token} with TTL
// matching session.ExpiresAt. Concurrently adds the token to the per-user
// sorted set for bulk revocation indexing (best-effort; failure is logged).
func (s *RedisSessionStore) Store(ctx context.Context, session *auth.Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("session store: marshal: %w", err)
	}

	ttl := session.TTL(time.Now())
	if ttl <= 0 {
		return fmt.Errorf("session store: session already expired")
	}

	if err := s.rdb.Set(ctx, sessionKey(session.Token), data, ttl).Err(); err != nil {
		return fmt.Errorf("session store: SET: %w", err)
	}

	// Best-effort: add to user session index so bulk revocation can find this token.
	// Score = expiry unix timestamp; enables ZREMRANGEBYSCORE pruning of stale entries.
	//
	// Service accounts (ServiceAccountID != uuid.Nil) have UserID == uuid.Nil.
	// Indexing service account sessions under uuid.Nil would cause all service
	// accounts to share one index key, making bulk revocation by user meaningless
	// and creating a large noisy index. Service account tokens are revoked via
	// is_revoked on iam_api_tokens, not via this user session index.
	if session.ServiceAccountID == (uuid.UUID{}) {
		indexKey := userIndexKey(session.TenantID, session.UserID)
		z := &goredis.Z{Score: float64(session.ExpiresAt.Unix()), Member: session.Token}
		if err := s.rdb.ZAdd(ctx, indexKey, z).Err(); err != nil {
			slog.Warn("session store: ZADD user index failed — bulk revocation impaired",
				"user_id", session.UserID,
				"tenant_id", session.TenantID,
				"error", err,
			)
		}
	}

	return nil
}

// Load retrieves and deserializes the session for the given token.
// Returns [auth.ErrSessionNotFound] when the key is absent (never issued,
// TTL-expired, or explicitly revoked). Returns a wrapped error for
// infrastructure failures; callers must NOT map these to 401.
func (s *RedisSessionStore) Load(ctx context.Context, token string) (*auth.Session, error) {
	data, err := s.rdb.Get(ctx, sessionKey(token)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, auth.ErrSessionNotFound
		}
		return nil, fmt.Errorf("session store: GET: %w", err)
	}

	var session auth.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("session store: unmarshal: %w", err)
	}

	return &session, nil
}

// Delete removes the session from the store. The primary DEL is critical:
// if it fails, Delete returns an error and the session remains live.
// The ZREM from the user index is best-effort: failure is logged, not returned.
func (s *RedisSessionStore) Delete(ctx context.Context, session *auth.Session) error {
	if err := s.rdb.Del(ctx, sessionKey(session.Token)).Err(); err != nil {
		return fmt.Errorf("session store: DEL session: %w", err)
	}

	// Best-effort: remove from user session index.
	// Skip for service account sessions — they are not indexed (see Store).
	if session.ServiceAccountID == (uuid.UUID{}) {
		indexKey := userIndexKey(session.TenantID, session.UserID)
		if err := s.rdb.ZRem(ctx, indexKey, session.Token).Err(); err != nil {
			slog.Warn("session store: ZREM user index failed — stale index entry remains",
				"user_id", session.UserID,
				"tenant_id", session.TenantID,
				"error", err,
			)
		}
	}

	return nil
}

// ListUserTokens prunes expired index entries then returns all non-expired
// token strings for the user. Called before DeleteAll during bulk revocation.
func (s *RedisSessionStore) ListUserTokens(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	indexKey := userIndexKey(tenantID, userID)

	// Prune expired entries (score ≤ now) before reading.
	// ZREMRANGEBYSCORE "-inf" <now_unix> removes all members whose score
	// (expiry timestamp) has elapsed. Best-effort — failure does not block the read.
	nowScore := fmt.Sprintf("%d", time.Now().Unix())
	if err := s.rdb.ZRemRangeByScore(ctx, indexKey, "-inf", nowScore).Err(); err != nil {
		slog.Warn("session store: ZREMRANGEBYSCORE prune failed — expired entries remain in index",
			"user_id", userID,
			"tenant_id", tenantID,
			"error", err,
		)
	}

	tokens, err := s.rdb.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("session store: ZRANGE user index: %w", err)
	}

	return tokens, nil
}

// DeleteAll removes all sessions in tokens and the user's index key.
// This is the bulk revocation path: called after role changes and forced logout.
// The index DEL is included in the same multi-key Del call as the session keys.
func (s *RedisSessionStore) DeleteAll(ctx context.Context, tenantID, userID uuid.UUID, tokens []string) error {
	// Build key list: one per session + the index set itself.
	keys := make([]string, 0, len(tokens)+1)
	for _, tok := range tokens {
		keys = append(keys, sessionKey(tok))
	}
	keys = append(keys, userIndexKey(tenantID, userID))

	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("session store: DEL sessions: %w", err)
	}

	return nil
}

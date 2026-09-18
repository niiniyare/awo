// Package redis — session_store.go
//
// RedisSessionStore implements [auth.SessionStore] using go-redis sorted sets
// for the per-user session index and plain string keys for session payloads.
//
// # Key layout
//
//	session:{token}                      — JSON-encoded auth.Session; TTL = ExpiresAt
//	session:revoked:{token}              — revocation tombstone; TTL bounds its lifetime (see IsRevoked)
//	user_sessions:{tenantID}:{userID}    — sorted set; member = token, score = expiry unix
//
// The sorted set enables O(log N) pruning of expired members via ZREMRANGEBYSCORE
// and O(N) enumeration for bulk revocation. This is the only data structure that
// supports expiry-aware enumeration without a full key scan.
//
// # Error handling
//
// Session key writes (SET, DEL on primary key), revocation tombstone writes,
// and IsRevoked reads are critical path — errors are returned to callers.
// Index writes (ZADD, ZREM, ZREMRANGEBYSCORE) are best-effort — errors are
// logged via slog but do not cause the method to fail. This asymmetry is
// intentional: a missing index entry impairs bulk revocation but does not
// compromise session validity or security. A missing tombstone write, by
// contrast, would weaken the session-resurrection defense (see IsRevoked in
// awo/auth), so it is treated as a hard failure like the primary DEL.
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

// revokedKey returns the Redis key for a session's revocation tombstone.
// Format: "session:revoked:{token}"
func revokedKey(token string) string {
	return "session:revoked:" + token
}

// bulkRevokeTombstoneTTL bounds the tombstone lifetime written by DeleteAll,
// which (unlike Delete) does not have each token's exact ExpiresAt on hand —
// only the raw token strings returned by ListUserTokens. Using the maximum
// possible human-session lifetime as the TTL is conservative (it can only
// over-protect, by outliving the token's actual, possibly-shorter, remaining
// validity — it can never under-protect) and needs no additional Redis round
// trip to look up each token's real expiry. Must be >= the largest session
// TTL issued anywhere in the framework; platform/iam's humanSessionTTL is
// currently 24h, so this intentionally matches it.
const bulkRevokeTombstoneTTL = 24 * time.Hour

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

// Delete removes the session from the store and writes a revocation
// tombstone (see IsRevoked). The primary DEL and the tombstone write are
// both critical: if either fails, Delete returns an error. The ZREM from the
// user index is best-effort: failure is logged, not returned.
func (s *RedisSessionStore) Delete(ctx context.Context, session *auth.Session) error {
	if err := s.rdb.Del(ctx, sessionKey(session.Token)).Err(); err != nil {
		return fmt.Errorf("session store: DEL session: %w", err)
	}

	// Tombstone TTL matches the session's own remaining lifetime — there is
	// nothing left to protect against once the session would have expired
	// naturally anyway (recoverSessionFromDB's own query already excludes
	// expired rows). A session with no remaining TTL needs no tombstone.
	if ttl := session.TTL(time.Now()); ttl > 0 {
		if err := s.rdb.Set(ctx, revokedKey(session.Token), "1", ttl).Err(); err != nil {
			return fmt.Errorf("session store: SET revocation tombstone: %w", err)
		}
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

// IsRevoked reports whether token has an active revocation tombstone written
// by a prior Delete or DeleteAll call. See [auth.SessionStore.IsRevoked].
func (s *RedisSessionStore) IsRevoked(ctx context.Context, token string) (bool, error) {
	n, err := s.rdb.Exists(ctx, revokedKey(token)).Result()
	if err != nil {
		return false, fmt.Errorf("session store: EXISTS revocation tombstone: %w", err)
	}
	return n > 0, nil
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

// DeleteAll removes all sessions in tokens and the user's index key, and
// writes a revocation tombstone (see IsRevoked) for each token, bounded by
// bulkRevokeTombstoneTTL since per-token exact expiry is not available here.
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

	if len(tokens) > 0 {
		pipe := s.rdb.Pipeline()
		for _, tok := range tokens {
			pipe.Set(ctx, revokedKey(tok), "1", bulkRevokeTombstoneTTL)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("session store: SET revocation tombstones: %w", err)
		}
	}

	return nil
}

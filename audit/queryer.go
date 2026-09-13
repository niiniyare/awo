package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

// HistoryEntry is a single audit event in a document's timeline.
// It is the response element for the GET /:id/history endpoint.
type HistoryEntry struct {
	ID            uuid.UUID      `json:"id"`
	ActorID       *uuid.UUID     `json:"actor_id,omitempty"`
	Action        string         `json:"action"`
	OccurredAt    time.Time      `json:"occurred_at"`
	ChangedFields []string       `json:"changed_fields,omitempty"`
	Before        map[string]any `json:"before,omitempty"`
	After         map[string]any `json:"after,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// Queryer retrieves audit history for a specific entity record.
// Implementations must be safe for concurrent use.
type Queryer interface {
	// History returns audit entries for the given entity record in
	// chronological order (oldest first). limit 0 means no server-side limit.
	// Uses the RLS context already set on the connection — no explicit
	// tenant_id filter is required.
	History(ctx context.Context, entityName string, recordID uuid.UUID, limit int) ([]HistoryEntry, error)
}

// NoopQueryer returns empty history without error.
// Use as the default when audit querying is not configured (e.g. dev/test).
type NoopQueryer struct{}

// History implements Queryer. Always returns nil, nil.
func (NoopQueryer) History(_ context.Context, _ string, _ uuid.UUID, _ int) ([]HistoryEntry, error) {
	return nil, nil
}

// PoolQueryer is a Queryer backed directly by a pgxpool.Pool.
// Construct one with NewPoolQueryer and pass it to router.RegisterOptions.AuditQueryer.
type PoolQueryer struct {
	pool *pgxpool.Pool
}

// NewPoolQueryer returns a PoolQueryer backed by pool.
// Panics if pool is nil to surface misconfiguration early.
func NewPoolQueryer(pool *pgxpool.Pool) *PoolQueryer {
	if pool == nil {
		panic("audit.NewPoolQueryer: pool must not be nil")
	}
	return &PoolQueryer{pool: pool}
}

// History queries platform_audit_log for all entries matching (entityName,
// recordID) ordered chronologically (oldest first).
//
// RLS on the connection already restricts results to the current tenant —
// no explicit tenant_id filter is needed or added here.
//
// limit 0 means no LIMIT clause is added; the caller receives all matching
// rows. Use a non-zero limit for UI pagination (default: 100, max: 500
// enforced by the HTTP handler).
func (q *PoolQueryer) History(ctx context.Context, entityName string, recordID uuid.UUID, limit int) ([]HistoryEntry, error) {
	return queryHistory(ctx, q.pool, entityName, recordID, limit)
}

// queryHistory is the shared SQL execution for all Queryer implementations.
func queryHistory(ctx context.Context, pool *pgxpool.Pool, entityName string, recordID uuid.UUID, limit int) ([]HistoryEntry, error) {
	query := `
SELECT
    id,
    actor_id,
    operation,
    created_at,
    changed_fields,
    before_data,
    after_data,
    context
FROM platform_audit_log
WHERE entity_name = $1
  AND record_id   = $2
ORDER BY created_at ASC`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := pool.Query(ctx, query, entityName, recordID)
	if err != nil {
		return nil, fmt.Errorf("audit.Queryer.History: query: %w", err)
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		var actorID *uuid.UUID
		var changedFields []string
		var beforeRaw, afterRaw, metaRaw []byte

		if err := rows.Scan(
			&e.ID,
			&actorID,
			&e.Action,
			&e.OccurredAt,
			&changedFields,
			&beforeRaw,
			&afterRaw,
			&metaRaw,
		); err != nil {
			return nil, fmt.Errorf("audit.Queryer.History: scan: %w", err)
		}
		e.ActorID = actorID
		e.ChangedFields = changedFields
		if len(beforeRaw) > 0 {
			if err := json.Unmarshal(beforeRaw, &e.Before); err != nil {
				return nil, fmt.Errorf("audit.Queryer.History: unmarshal before_data: %w", err)
			}
		}
		if len(afterRaw) > 0 {
			if err := json.Unmarshal(afterRaw, &e.After); err != nil {
				return nil, fmt.Errorf("audit.Queryer.History: unmarshal after_data: %w", err)
			}
		}
		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &e.Metadata); err != nil {
				return nil, fmt.Errorf("audit.Queryer.History: unmarshal context: %w", err)
			}
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit.Queryer.History: rows: %w", err)
	}
	return entries, nil
}

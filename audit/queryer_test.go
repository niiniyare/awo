package audit_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/audit"
)

// TestNoopQueryer_ReturnsEmpty verifies that NoopQueryer.History returns nil,
// nil for any input — it is the safe default when audit querying is disabled.
func TestNoopQueryer_ReturnsEmpty(t *testing.T) {
	q := audit.NoopQueryer{}
	entries, err := q.History(context.Background(), "finance_invoice", uuid.New(), 0)
	require.NoError(t, err)
	assert.Nil(t, entries)
}

// TestNoopQueryer_ZeroLimit verifies limit=0 is handled without error.
func TestNoopQueryer_ZeroLimit(t *testing.T) {
	q := audit.NoopQueryer{}
	entries, err := q.History(context.Background(), "iam_user", uuid.Nil, 0)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestHistoryEntry_JSONFields verifies that HistoryEntry serialises with the
// correct JSON field names as required by the API contract.
func TestHistoryEntry_JSONFields(t *testing.T) {
	actorID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	e := audit.HistoryEntry{
		ID:            uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		ActorID:       &actorID,
		Action:        "create",
		OccurredAt:    time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		ChangedFields: []string{"name", "status"},
		Before:        map[string]any{"status": "draft"},
		After:         map[string]any{"status": "submitted"},
		Metadata:      map[string]any{"ip": "127.0.0.1"},
	}

	b, err := json.Marshal(e)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "22222222-2222-2222-2222-222222222222", m["id"])
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", m["actor_id"])
	assert.Equal(t, "create", m["action"])
	assert.Equal(t, "2026-01-15T10:00:00Z", m["occurred_at"])
	assert.NotNil(t, m["changed_fields"])
	assert.NotNil(t, m["before"])
	assert.NotNil(t, m["after"])
	assert.NotNil(t, m["metadata"])
}

// TestHistoryEntry_NilActorID_Omitted verifies that actor_id is omitted from
// JSON output when nil — important for system-originated audit records.
func TestHistoryEntry_NilActorID_Omitted(t *testing.T) {
	e := audit.HistoryEntry{
		ID:         uuid.New(),
		ActorID:    nil, // system actor — must be omitted
		Action:     "system",
		OccurredAt: time.Now(),
	}

	b, err := json.Marshal(e)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))

	_, hasActorID := m["actor_id"]
	assert.False(t, hasActorID, "actor_id must be omitted when nil (omitempty)")
}

// TestHistoryEntry_EmptySlicesOmitted verifies that changed_fields,
// before, after, and metadata are omitted when empty/nil.
func TestHistoryEntry_EmptySlicesOmitted(t *testing.T) {
	e := audit.HistoryEntry{
		ID:         uuid.New(),
		Action:     "delete",
		OccurredAt: time.Now(),
	}

	b, err := json.Marshal(e)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))

	_, hasBefore := m["before"]
	_, hasAfter := m["after"]
	_, hasChanged := m["changed_fields"]
	_, hasMeta := m["metadata"]

	assert.False(t, hasBefore)
	assert.False(t, hasAfter)
	assert.False(t, hasChanged)
	assert.False(t, hasMeta)
}

// TestNoopQueryer_ImplementsQueryer verifies the interface is satisfied at
// compile time. If Queryer changes, this test will catch it.
func TestNoopQueryer_ImplementsQueryer(t *testing.T) {
	var _ audit.Queryer = audit.NoopQueryer{}
}

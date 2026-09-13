package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/driver"
)

// Entry is the structured input for writing an audit log record.
type Entry struct {
	TenantID   uuid.UUID
	ActorID    uuid.UUID
	ActorEmail string
	IPAddress  string
	RequestID  string
	Operation  string // "create", "update", "delete", "action", "login", "logout"
	EntityName string
	RecordID   uuid.UUID
	Before     any // serialized to JSON
	After      any // serialized to JSON
	Diff       map[string]any
	OccurredAt time.Time
}

// Writer appends entries to the iam_audit_log entity.
// Writes run inside the caller's transaction — errors cause rollback.
type Writer struct {
	repo driver.EntityRepository[*def.EntityRecord]
}

// NewWriter creates a Writer backed by the iam_audit_log repository.
func NewWriter(repo driver.EntityRepository[*def.EntityRecord]) *Writer {
	return &Writer{repo: repo}
}

// Write persists e as an audit log entry.
// Must be called inside a transaction — caller's transaction is used.
func (w *Writer) Write(ctx context.Context, e Entry) error {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}

	beforeJSON, err := marshalOptional(e.Before)
	if err != nil {
		return fmt.Errorf("audit.Write: marshal before: %w", err)
	}
	afterJSON, err := marshalOptional(e.After)
	if err != nil {
		return fmt.Errorf("audit.Write: marshal after: %w", err)
	}
	diffJSON, err := marshalOptional(e.Diff)
	if err != nil {
		return fmt.Errorf("audit.Write: marshal diff: %w", err)
	}

	_, err = w.repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{
			"tenant_id_ref":   e.TenantID.String(),
			"actor_id":        e.ActorID.String(),
			"actor_email":     e.ActorEmail,
			"ip_address":      e.IPAddress,
			"request_id":      e.RequestID,
			"operation":       e.Operation,
			"entity_name":     e.EntityName,
			"record_id":       nonZeroUUID(e.RecordID),
			"before_snapshot": beforeJSON,
			"after_snapshot":  afterJSON,
			"diff":            diffJSON,
		},
	})
	if err != nil {
		return fmt.Errorf("audit.Write: create: %w", err)
	}
	return nil
}

func marshalOptional(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func nonZeroUUID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

// ComputeDiff returns the keys that changed between before and after maps.
// Only top-level keys are compared; nested objects are compared as values.
func ComputeDiff(before, after map[string]any) map[string]any {
	diff := make(map[string]any)
	for k, newVal := range after {
		oldVal, exists := before[k]
		if !exists {
			diff[k] = map[string]any{"old": nil, "new": newVal}
			continue
		}
		// Simple comparison via JSON round-trip for consistent equality.
		oldJSON, _ := json.Marshal(oldVal)
		newJSON, _ := json.Marshal(newVal)
		if string(oldJSON) != string(newJSON) {
			diff[k] = map[string]any{"old": oldVal, "new": newVal}
		}
	}
	for k, oldVal := range before {
		if _, exists := after[k]; !exists {
			diff[k] = map[string]any{"old": oldVal, "new": nil}
		}
	}
	return diff
}

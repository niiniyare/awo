package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	. "awo.so/awo/events"
)

// --- DomainEvent construction ---

func TestDomainEvent_Fields(t *testing.T) {
	tenantID := uuid.New()
	recordID := uuid.New()
	actorID := uuid.New()
	now := time.Now().UTC()

	e := DomainEvent{
		ID:         uuid.New(),
		TenantID:   tenantID,
		Type:       EventCreated,
		EntityName: "finance_invoice",
		RecordID:   recordID,
		ActorID:    actorID,
		OccurredAt: now,
		Payload:    []byte(`{"status":"draft"}`),
	}

	if e.TenantID != tenantID {
		t.Errorf("TenantID mismatch")
	}
	if e.RecordID != recordID {
		t.Errorf("RecordID mismatch")
	}
	if e.Type != EventCreated {
		t.Errorf("Type mismatch")
	}
	if e.EntityName != "finance_invoice" {
		t.Errorf("EntityName mismatch")
	}
	if e.ActorID != actorID {
		t.Errorf("ActorID mismatch")
	}
	if string(e.Payload) != `{"status":"draft"}` {
		t.Errorf("Payload mismatch")
	}
}

// --- EventType constants ---

func TestEventType_Constants(t *testing.T) {
	types := map[EventType]string{
		EventCreated:     "entity.created",
		EventUpdated:     "entity.updated",
		EventDeleted:     "entity.deleted",
		EventActionFired: "entity.action",
	}
	for et, want := range types {
		if string(et) != want {
			t.Errorf("EventType %q: expected %q, got %q", et, want, string(et))
		}
	}
}

// --- NoopPublisher ---

func TestNoopPublisher_Publish_NoError(t *testing.T) {
	var p NoopPublisher
	err := p.Publish(context.Background(), DomainEvent{
		ID:         uuid.New(),
		Type:       EventCreated,
		EntityName: "some_entity",
		RecordID:   uuid.New(),
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("NoopPublisher.Publish should not error, got: %v", err)
	}
}

func TestNoopPublisher_ImplementsPublisher(t *testing.T) {
	var _ Publisher = NoopPublisher{}
}

// --- DomainEvent for action events ---

func TestDomainEvent_ActionEvent(t *testing.T) {
	e := DomainEvent{
		ID:         uuid.New(),
		Type:       EventActionFired,
		EntityName: "finance_payment",
		RecordID:   uuid.New(),
		ActionName: "submit",
		OccurredAt: time.Now().UTC(),
	}
	if e.Type != EventActionFired {
		t.Errorf("expected EventActionFired type")
	}
	if e.ActionName != "submit" {
		t.Errorf("expected ActionName='submit', got %q", e.ActionName)
	}
}

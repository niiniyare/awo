package audit

import (
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func TestAuditRecord_Validate(t *testing.T) {
	t.Parallel()

	base := func() AuditRecord {
		return AuditRecord{
			TenantID:      uuid.New(),
			EntityName:    "finance_invoice",
			Operation:     OperationCreate,
			EventCategory: CategoryData,
			Actor:         &def.Actor{UserID: uuid.New()},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*AuditRecord)
		wantErr bool
	}{
		{
			name:    "valid_actor",
			mutate:  func(_ *AuditRecord) {},
			wantErr: false,
		},
		{
			name: "valid_system_actor",
			mutate: func(r *AuditRecord) {
				r.Actor = nil
				r.SystemActor = SystemBootstrap
			},
			wantErr: false,
		},
		{
			name: "missing_tenant_id",
			mutate: func(r *AuditRecord) {
				r.TenantID = uuid.Nil
			},
			wantErr: true,
		},
		{
			name: "missing_entity_name",
			mutate: func(r *AuditRecord) {
				r.EntityName = ""
			},
			wantErr: true,
		},
		{
			name: "missing_operation",
			mutate: func(r *AuditRecord) {
				r.Operation = ""
			},
			wantErr: true,
		},
		{
			name: "missing_event_category",
			mutate: func(r *AuditRecord) {
				r.EventCategory = ""
			},
			wantErr: true,
		},
		{
			name: "both_actor_and_system_actor",
			mutate: func(r *AuditRecord) {
				r.SystemActor = SystemBootstrap
			},
			wantErr: true,
		},
		{
			name: "neither_actor_nor_system_actor",
			mutate: func(r *AuditRecord) {
				r.Actor = nil
				r.SystemActor = ""
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := base()
			tc.mutate(&rec)
			err := rec.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestAuditRecord_ActorID(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	rec := AuditRecord{Actor: &def.Actor{UserID: userID}}
	if got := rec.ActorID(); got != userID {
		t.Errorf("ActorID() = %v, want %v", got, userID)
	}

	// System actor — ActorID must be uuid.Nil.
	sysRec := AuditRecord{SystemActor: SystemOutboxRelay}
	if got := sysRec.ActorID(); got != uuid.Nil {
		t.Errorf("ActorID() for system actor = %v, want uuid.Nil", got)
	}
}

func TestAuditRecord_ServiceAccountID(t *testing.T) {
	t.Parallel()

	saID := uuid.New()
	rec := AuditRecord{Actor: &def.Actor{ServiceAccountID: saID}}
	if got := rec.ServiceAccountID(); got != saID {
		t.Errorf("ServiceAccountID() = %v, want %v", got, saID)
	}

	// System actor — ServiceAccountID must be uuid.Nil.
	sysRec := AuditRecord{SystemActor: SystemScheduler}
	if got := sysRec.ServiceAccountID(); got != uuid.Nil {
		t.Errorf("ServiceAccountID() for system actor = %v, want uuid.Nil", got)
	}
}

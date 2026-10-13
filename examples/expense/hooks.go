package expense

import (
	"context"

	"awo.so/awo/def"
	"awo.so/awo/runtime"
)

// validTransitions enumerates the only status changes StatusTransitionGuard
// permits. Any (from, to) pair absent from this map — including every
// same-status "no-op" update that doesn't touch status at all, which never
// reaches the check below — is rejected.
var validTransitions = map[string]map[string]bool{
	"draft":     {"submitted": true},
	"submitted": {"approved": true, "rejected": true},
	"rejected":  {"draft": true},
	// "approved" is terminal: no outgoing transitions.
}

// StatusTransitionGuard enforces the expense lifecycle:
//
//	draft -> submitted -> approved
//	                   \-> rejected -> draft (resubmit)
//
// Deliberately a small, explicit BeforeUpdate hook rather than a new
// workflow/state-machine subsystem. Deliberately does not check *who* may
// perform a given transition (e.g. "only a manager may approve") — that is
// an authorization concern (def.PermissionSet / Casbin), not this hook's
// job, and is out of scope for this showcase.
//
// Parameter order note (verified against runtime/pipeline.go's actual call
// site, runtime.Pipeline.RunBeforeUpdate: "return h.BeforeUpdate(pctx.Ctx,
// proposed, current)" — proposed is the patch merged over the existing
// record, current is the unpatched, already-persisted record read moments
// earlier by EntityService.Update's own s.repo.Get): the FIRST parameter is
// always the proposed/new record, the SECOND is always the prior/current
// one. platform/organization.MoveGuard's existing BeforeUpdate names its own
// parameters the other way around ("current, incoming") while the compiler
// still binds them positionally to (proposed, current) regardless of name —
// a real, pre-existing bug there (see the technical findings register),
// deliberately not repeated here.
type StatusTransitionGuard struct{}

func (g *StatusTransitionGuard) BeforeUpdate(_ context.Context, proposed, current *def.EntityRecord) error {
	newStatus, changing := proposed.Data["status"].(string)
	if !changing {
		return nil // status not part of this update at all
	}
	oldStatus, _ := current.Data["status"].(string)
	if newStatus == oldStatus {
		return nil
	}
	if validTransitions[oldStatus][newStatus] {
		return nil
	}
	return &runtime.BusinessError{
		Code:    "expense.invalid_status_transition",
		Message: "cannot move expense from \"" + oldStatus + "\" to \"" + newStatus + "\"",
		Status:  409,
	}
}

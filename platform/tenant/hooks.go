package tenant

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"awo.so/awo/def"
	"awo.so/awo/runtime"
)

// slugRe is the allowed pattern for tenant slugs.
// Slugs are lowercase letters, digits, and hyphens; must start with a letter.
var slugRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,61}[a-z0-9]$`)

// validTransitions maps current status → set of allowed next statuses.
var validTransitions = map[string]map[string]bool{
	"PENDING":   {"ACTIVE": true, "ARCHIVED": true},
	"ACTIVE":    {"SUSPENDED": true, "ARCHIVED": true},
	"SUSPENDED": {"ACTIVE": true, "ARCHIVED": true},
	"ARCHIVED":  {}, // terminal
}

// SlugValidator enforces slug format before create.
type SlugValidator struct{}

func (v *SlugValidator) BeforeCreate(_ context.Context, rec *def.EntityRecord) error {
	slug := strings.TrimSpace(rec.GetString("slug"))

	if !slugRe.MatchString(slug) {
		return &runtime.ValidationError{
			Fields: map[string]string{
				"slug": "must be 3–63 lowercase letters, digits, or hyphens; must start with a letter",
			},
		}
	}
	return nil
}

// StatusValidator ensures initial status is PENDING on create.
type StatusValidator struct{}

func (v *StatusValidator) BeforeCreate(_ context.Context, rec *def.EntityRecord) error {
	status := rec.GetString("status")
	if status != "" && status != "PENDING" {
		return &runtime.BusinessError{
			Code:    "tenant.invalid_initial_status",
			Message: fmt.Sprintf("New tenants must start as PENDING, not %q", status),
			Status:  422,
		}
	}
	rec.Set("status", "PENDING")
	return nil
}

// TransitionGuard enforces the tenant lifecycle state machine on update.
type TransitionGuard struct{}

func (g *TransitionGuard) BeforeUpdate(_ context.Context, rec *def.EntityRecord, prev *def.EntityRecord) error {
	newStatus := rec.GetString("status")
	if newStatus == "" {
		return nil // not changing status
	}
	currentStatus := prev.GetString("status")
	if newStatus == currentStatus {
		return nil
	}

	allowed, ok := validTransitions[currentStatus]
	if !ok {
		return &runtime.BusinessError{
			Code:    "tenant.unknown_status",
			Message: fmt.Sprintf("Unknown current status %q", currentStatus),
			Status:  422,
		}
	}
	if !allowed[newStatus] {
		return &runtime.BusinessError{
			Code:    "tenant.invalid_transition",
			Message: fmt.Sprintf("Cannot transition tenant from %s to %s", currentStatus, newStatus),
			Status:  422,
		}
	}
	return nil
}

package organization

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"awo.so/awo/def"
	"awo.so/awo/runtime"
)

var codePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,49}$`)

// CodeValidator enforces that organization codes follow the uppercase
// alphanumeric pattern. Codes are immutable after creation (def.Immutable)
// so this check only runs on BeforeCreate.
type CodeValidator struct{}

func (v *CodeValidator) BeforeCreate(_ context.Context, r *def.EntityRecord) error {
	code, _ := r.Data["code"].(string)
	if code == "" {
		return &runtime.ValidationError{Fields: map[string]string{
			"code": "required",
		}}
	}
	if !codePattern.MatchString(code) {
		return &runtime.ValidationError{Fields: map[string]string{
			"code": "must be uppercase alphanumeric (A-Z, 0-9, hyphens, underscores), starting with a letter or digit",
		}}
	}
	return nil
}

// PathComputeHook derives the materialized path and depth from the parent_id
// field before the record is persisted. If parent_id is absent the record
// is a root node (path = "/<self-id>/", depth = 0).
//
// Cycle prevention: if the parent's path already contains the current record's
// ID then a cycle would be formed; the hook rejects the operation.
//
// NOTE: In the stub implementation the parent's path is fetched from
// r.Data["_parent_path"] if the caller pre-populates it. A real implementation
// injects an EntityRepository and fetches the parent record directly.
type PathComputeHook struct{}

func (h *PathComputeHook) BeforeCreate(_ context.Context, r *def.EntityRecord) error {
	parentID, _ := r.Data["parent_id"].(string)
	selfID := r.ID.String()

	if parentID == "" {
		// Root node.
		r.Data["path"] = "/" + selfID + "/"
		r.Data["depth"] = 0
		return nil
	}

	// In a real implementation: fetch parent, read its path + depth.
	// Here we read from _parent_path if the caller pre-populated it (testing hook).
	parentPath, _ := r.Data["_parent_path"].(string)
	if parentPath == "" {
		// TODO: inject EntityRepository and fetch parent.path here.
		r.Data["path"] = "/" + selfID + "/"
		r.Data["depth"] = 0
		return nil
	}

	// Cycle guard: parent's path must not contain selfID.
	if strings.Contains(parentPath, "/"+selfID+"/") {
		return fmt.Errorf("organization.PathComputeHook: cycle detected — self ID already present in parent path")
	}

	parentDepth, _ := r.Data["_parent_depth"].(int)
	r.Data["path"] = parentPath + selfID + "/"
	r.Data["depth"] = parentDepth + 1

	// Remove internal transport fields.
	delete(r.Data, "_parent_path")
	delete(r.Data, "_parent_depth")

	return nil
}

// MoveGuard prevents direct writes to path and depth fields via the standard
// Update pathway. Moves must go through OrganizationService.Move which
// recomputes path and depth atomically for the subtree.
type MoveGuard struct{}

func (g *MoveGuard) BeforeUpdate(_ context.Context, current, incoming *def.EntityRecord) error {
	if _, ok := incoming.Data["path"]; ok {
		return fmt.Errorf("organization.MoveGuard: path is managed by OrganizationService.Move — do not set directly")
	}
	if _, ok := incoming.Data["depth"]; ok {
		return fmt.Errorf("organization.MoveGuard: depth is managed by OrganizationService.Move — do not set directly")
	}
	return nil
}

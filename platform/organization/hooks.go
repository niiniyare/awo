package organization

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/driver"
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

// PathComputeHook derives the materialized path and depth of an organization
// node from its actual persisted parent record, immediately after creation.
// If parent_id is absent the record is a root node (path = "/<self-id>/",
// depth = 0).
//
// Why AfterCreate, not BeforeCreate: the record's real ID does not exist
// until contrib/pgx.Repository.Create generates it (uuid.New(), internally)
// — runtime.Pipeline.RunBeforeCreate's assembled record.ID is never passed
// into driver.CreateInput and is discarded. A BeforeCreate hook therefore
// cannot compute a self-referencing path (root or non-root) correctly; by
// the time AfterCreate runs, record carries the real, persisted ID and the
// operation is still inside the same database transaction (HookSet: AfterCreate
// hooks run "INSIDE TX... error causes rollback"), so a parent lookup and the
// corrective path/depth write below are atomic with the record's own
// creation. The "/" path default on the field (definition.go) only exists to
// satisfy the NOT NULL constraint for the instant between INSERT and this
// hook's correction.
//
// Cycle prevention: on Create this is structurally unreachable (a brand-new
// ID cannot already appear in any existing path) but is kept as a cheap
// defensive check in case of ID reuse elsewhere.
//
// repo must be wired via WireRepository before any organization is created;
// see that function's doc comment.
type PathComputeHook struct {
	repo driver.EntityRepository[*def.EntityRecord]
}

// WireRepository supplies the platform_organization repository that
// PathComputeHook uses to look up parent records. Must be called once during
// bootstrap, after the repository's backing pool exists and before any HTTP
// traffic that could create an organization is accepted (see
// cmd/server/main.go, mirroring the existing contrib.NewRepository(pool,
// schema) pattern used for iam/tenant). Not safe to call concurrently with
// organization creation; intended for single-threaded startup wiring only.
func WireRepository(repo driver.EntityRepository[*def.EntityRecord]) {
	pathComputeHook.repo = repo
}

// pathComputeHook is the single PathComputeHook instance referenced by
// Definition.Hooks.AfterCreate (definition.go) and mutated in place by
// WireRepository — Definition is a package-level var populated at init()
// time, before any database pool exists, so the repository dependency must
// be late-bound rather than passed to a constructor.
var pathComputeHook = &PathComputeHook{}

// recordUUID extracts a uuid.UUID from an EntityRecord.Data value of any
// shape contrib/pgx's generic any-scan produces for a uuid column: a plain
// string (what a caller supplies on Create, before persistence), a
// pgx-scanned [16]byte/[16]uint8 (what Repository.Get/getWithDB returns —
// pgx's default uuid-column decode target for a bare `any` destination,
// confirmed empirically against this table), or a uuid.UUID itself. Returns
// ok=false for nil/absent/empty values (the "no parent" case) and for any
// other shape, which callers must treat as a parse failure, never as empty.
func recordUUID(v any) (id uuid.UUID, ok bool, malformed bool) {
	switch t := v.(type) {
	case nil:
		return uuid.Nil, false, false
	case uuid.UUID:
		return t, t != uuid.Nil, false
	case [16]byte:
		id := uuid.UUID(t)
		return id, id != uuid.Nil, false
	case string:
		if t == "" {
			return uuid.Nil, false, false
		}
		parsed, err := uuid.Parse(t)
		if err != nil {
			return uuid.Nil, false, true
		}
		return parsed, true, false
	default:
		return uuid.Nil, false, true
	}
}

// recordInt64 extracts an integer from an EntityRecord.Data value of any
// shape a Postgres integer/bigint column scans into via contrib/pgx's
// generic any-scan (int32 for the hand-maintained platform_organization
// migration's INTEGER depth column; int64 would appear here too if that
// column were ever changed to BIGINT, matching def.FieldTypeInt's general
// "→ int64" convention elsewhere in the framework).
func recordInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int:
		return int64(t)
	default:
		return 0
	}
}

func (h *PathComputeHook) AfterCreate(ctx context.Context, r *def.EntityRecord) error {
	if h.repo == nil {
		return fmt.Errorf("organization.PathComputeHook: repository not wired — call organization.WireRepository at bootstrap before accepting traffic")
	}

	selfID := r.ID.String()
	parentID, hasParent, malformed := recordUUID(r.Data["parent_id"])
	if malformed {
		return fmt.Errorf("organization.PathComputeHook: invalid parent_id %#v", r.Data["parent_id"])
	}

	var path string
	var depth int64
	if !hasParent {
		// Root node.
		path = "/" + selfID + "/"
		depth = 0
	} else {
		// Get runs inside the active transaction (ctx carries it — see
		// contrib/pgx.Repository.WithTx/connFromContext) under the standard
		// tenant RLS policy. A parent in another tenant is filtered out by
		// RLS exactly like a nonexistent parent — both surface as
		// driver.ErrNotFound here and are rejected identically, never
		// silently treated as a root node and never distinguished in a way
		// that would disclose cross-tenant existence.
		parent, err := h.repo.Get(ctx, parentID)
		if err != nil {
			return fmt.Errorf("organization.PathComputeHook: parent organization %s: %w", parentID, err)
		}

		parentPath, _ := parent.Data["path"].(string)
		parentDepth := recordInt64(parent.Data["depth"])

		// Defensive cycle guard — see type doc comment; unreachable on
		// Create in practice since selfID is freshly generated.
		if strings.Contains(parentPath, "/"+selfID+"/") {
			return fmt.Errorf("organization.PathComputeHook: cycle detected — self ID already present in parent path")
		}

		path = parentPath + selfID + "/"
		depth = parentDepth + 1
	}

	if _, err := h.repo.Update(ctx, r.ID, driver.UpdateInput{
		Data: map[string]any{"path": path, "depth": depth},
	}); err != nil {
		return fmt.Errorf("organization.PathComputeHook: persist computed path: %w", err)
	}

	// Keep the in-memory record (what EntityService returns, audits, and
	// publishes as a lifecycle event) consistent with what was just persisted.
	r.Data["path"] = path
	r.Data["depth"] = depth
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

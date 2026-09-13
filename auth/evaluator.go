package auth

import "context"

// PolicyEvaluator is the single enforcement point for all capability decisions
// in the Awo Framework (ADR-001). The runtime consults it at the AUTHORIZE
// pipeline stage for every mutation and before every route handler.
//
// The default implementation is Casbin-backed ([NewCasbinEvaluator]). It can
// be replaced with OPA, ReBAC, or a custom engine by providing an alternative
// implementation at startup — no EntityDefinition changes are required.
//
// Implementations MUST be goroutine-safe. The evaluator is shared across all
// concurrent requests and tenants; no per-request state should be held.
//
// Implementations MUST be fast. CanPerform is called on every authenticated
// request; database I/O on the hot path is prohibited. Policy data MUST be
// pre-loaded and held in memory, refreshed via background reload.
//
// Authorization flow:
//
//  1. Middleware constructs ViewerContext from the validated Session.
//  2. viewer.IsPlatformAdmin() == true → AUTHORIZE stage is skipped entirely.
//  3. PolicyEvaluator.CanPerform(ctx, viewer, qualifiedEntityName, action):
//     - (true,  nil)   → proceed to pipeline
//     - (false, nil)   → HTTP 403 PermissionError
//     - (_,     error) → HTTP 500 internal error
type PolicyEvaluator interface {
	// CanPerform returns true if the viewer may perform action on object.
	//
	// object is the qualified entity name (e.g. "finance_invoice", "iam_user").
	// action is one of the standard operations ("create", "read", "update",
	// "delete") or a custom action name declared in ActionDef.Name (e.g.
	// "submit", "approve", "cancel").
	//
	// Implementations MUST return (false, nil) — not an error — when the
	// viewer simply lacks the required permission. Errors are reserved for
	// infrastructure failures (e.g. the policy store is unreachable).
	//
	// Implementations MUST NOT perform database I/O; policy data must be
	// pre-loaded in memory (ADR-001 enforcement layer contract).
	CanPerform(ctx context.Context, viewer ViewerContext, object string, action string) (bool, error)
}

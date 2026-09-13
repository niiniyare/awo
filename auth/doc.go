// Package auth provides the enforcement layer of Awo's two-layer authorization
// architecture (ADR-001).
//
// # Architecture
//
// Authorization in Awo is split into two distinct, independently replaceable
// layers:
//
//   - Declaration layer (awo/def): [def.PermissionSet] on EntityDefinition.
//     Module authors declare permission identifiers — stable, engine-agnostic
//     capability names (e.g. "finance.invoice.create"). No roles, no subjects,
//     no backend constructs.
//
//   - Enforcement layer (awo/auth, this package): [PolicyEvaluator] interface.
//     The runtime consults it on every request. The default implementation is
//     Casbin-backed. It can be replaced with OPA, ReBAC, or a custom engine
//     without touching any EntityDefinition.
//
// # ViewerContext
//
// Every request context carries a [ViewerContext] that identifies the
// authenticated principal. The middleware pipeline constructs it from the
// validated [Session] and injects it via [WithViewer]. Downstream code
// retrieves it via [ViewerFromContext], which panics if absent — a
// middleware guarantee violation is a programming error, not a runtime failure.
//
// # Session
//
// [Session] is the frozen struct (ADR-004) stored in Redis at login. It
// carries all claims needed to construct a [ViewerContext]. Sessions are
// validated on every request; expiry and tenant status are checked before
// any handler runs.
//
// # PolicyEvaluator
//
// [PolicyEvaluator] is the single enforcement point for all capability
// decisions. The runtime consults it at the AUTHORIZE pipeline stage before
// every mutation and route handler. The default Casbin implementation loads
// [compiler.CapabilityGrant] records at startup and pairs them with
// role-to-permission mappings from the IAM module.
//
// Platform admins ("role:platform-admin") bypass [PolicyEvaluator] entirely;
// the middleware short-circuits before consulting it.
package auth

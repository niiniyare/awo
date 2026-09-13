// Package def is the frozen kernel of the Awo framework.
//
// It declares every type that module authors and framework subsystems share:
// EntityDefinition, FieldDef, EdgeDef, HookSet, PolicyFunc, ActionDef,
// WorkflowTrigger, PermissionSet, and PageBuilderSet.
//
// # Stability
//
// This package is FROZEN. No breaking changes are permitted after the 1.0
// release. Additive changes (new fields with zero values, new constants) are
// allowed. Removals and type changes are not.
//
// # Dependency rule
//
// def has zero framework dependencies. It imports only the Go standard library
// and two stable third-party value-object libraries:
//   - github.com/google/uuid
//   - github.com/shopspring/decimal
//
// No other framework package may be imported here. This keeps the dependency
// graph acyclic: everything else imports def, never the reverse.
//
// # Usage
//
// Module authors declare entity shapes in their package, then call
// [Register] from an init() function:
//
//	var InvoiceDefinition = def.SystemDefinition{
//	    Name:   "finance_invoice",
//	    Module: "finance",
//	    // ...
//	}
//
//	func init() {
//	    def.Register(&InvoiceDefinition)
//	}
//
// The registry, compiler, and runtime subsystems read the accumulated
// registrations at startup — never at request time.
package def

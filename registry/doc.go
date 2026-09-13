// Package registry implements the Entity Registry: the subsystem that
// accumulates [def.EntityDefinition] registrations, validates them, and
// seals the registry before the compiler runs.
//
// # Lifecycle
//
//  1. All module init() functions call [def.Register], populating the global
//     def registry.
//  2. During the Initialization Phase, [Build] is called exactly once. It
//     reads all registrations from the def package, validates each definition,
//     and returns a sealed [Registry].
//  3. The sealed Registry is passed to the compiler and the runtime. No
//     further registrations are accepted after [Build] returns.
//
// # Validation
//
// The registry enforces the following rules on registration:
//   - Entity name must be non-empty, snake_case, format {module}_{noun}.
//   - Entity name must be unique across all registered definitions.
//   - Field names must be unique within a definition.
//   - FieldTypeLink LinkTarget must reference a registered entity.
//   - Mandatory entity names (ledger_entry, stock_move, payment, etc.) must
//     be declared as SystemDefinition, not CustomDefinition.
//   - No two entities in the same module may share a Label.
//
// Validation failures during [Build] are fatal — the process exits because
// serving requests without a valid compiled schema is undefined behaviour.
package registry

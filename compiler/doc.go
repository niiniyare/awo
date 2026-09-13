// Package compiler implements the Compilation Phase of the Awo framework.
//
// # Responsibility
//
// The compiler transforms a sealed [registry.Registry] into a [CompiledSchema]
// that all runtime subsystems consume. It runs once during startup, before the
// first request is accepted, and produces read-only output.
//
// # What the compiler produces
//
// For each EntityDefinition, the compiler produces:
//   - [EntitySchema]: derived metadata used at request time (field map by name,
//     sorted field/edge lists, resolved link targets, default value functions)
//   - SQL DDL statements for migration generation (informational; not executed)
//   - Casbin policy assertions derived from PermissionSet
//   - Route descriptors used by the API layer to register Fiber routes
//   - amis schema templates used as the cache-miss fallback for PageBuilder
//
// # Compiler phases
//
//  1. Semantic analysis — validates cross-entity references and type constraints
//     (this overlaps with registry validation; the compiler catches runtime-only
//     issues like circular edge references).
//  2. IR construction — builds the intermediate representation from each def.
//  3. Output emission — produces CompiledSchema structs for runtime consumption.
//
// # Usage
//
//	reg := registry.Build()
//	schema, err := compiler.Compile(reg)
//	if err != nil {
//	    log.Fatal("compilation failed", "err", err)
//	}
package compiler

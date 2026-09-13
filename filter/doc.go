// Package filter implements the declarative predicate DSL used throughout the
// Awo framework for querying entity data.
//
// # Design
//
// A [Filter] is an immutable tree of predicates. Leaf nodes are field
// comparisons (Eq, Gt, Contains, etc.). Branch nodes are logical operators
// (And, Or, Not). The runtime translates a Filter tree into parameterised SQL
// at request time.
//
// Filters never contain raw SQL or user-supplied strings in query position —
// only field names (validated against the entity's FieldDef at compile time)
// and typed values (bound as SQL parameters). This eliminates SQL injection
// by construction.
//
// # Usage
//
//	import "awo.so/awo/filter"
//
//	// Simple equality
//	f := filter.Eq("status", "Draft")
//
//	// Compound predicate
//	f := filter.And(
//	    filter.Eq("status", "Draft"),
//	    filter.Gt("total_kes", decimal.NewFromInt(0)),
//	)
//
//	// Negation
//	f := filter.Not(filter.Eq("archived", true))
//
// # PolicyFunc integration
//
// PolicyFuncs in the def package declare their return type as [def.Filter],
// an interface. This package's [*Filter] implements that interface, allowing
// policy functions to use the full DSL without def importing this package.
package filter

package def

import (
	"fmt"
	"sync"
)

// globalRegistry holds all EntityDefinitions registered via [Register].
// It is populated exclusively from init() functions — never from request
// handlers or goroutines. The registry is sealed (read-only) once [Seal] is
// called during the Compilation Phase.
var globalRegistry = &entityRegistry{}

type entityRegistry struct {
	mu     sync.RWMutex
	defs   []EntityDefinition
	byName map[string]EntityDefinition
	sealed bool
}

// Register records an EntityDefinition in the global registry. It must be
// called from an init() function — never from request handlers or goroutines.
//
// Register panics if:
//   - def is nil
//   - def.EntityName() is empty
//   - another definition with the same name is already registered
//   - the registry has already been sealed (compilation has begun)
//
// The panic-on-error contract is intentional: registration errors are
// programming mistakes that must be caught at startup, not runtime.
func Register(def EntityDefinition) {
	if def == nil {
		panic("def.Register: nil EntityDefinition")
	}
	if def.EntityName() == "" {
		panic("def.Register: EntityDefinition has empty Name")
	}
	if def.EntityModule() == "" {
		panic(fmt.Sprintf("def.Register: entity %q has empty Module", def.EntityName()))
	}
	qname := QualifiedName(def)

	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if globalRegistry.sealed {
		panic(fmt.Sprintf("def.Register: cannot register %q after the registry is sealed", qname))
	}
	if globalRegistry.byName == nil {
		globalRegistry.byName = make(map[string]EntityDefinition)
	}
	if existing, ok := globalRegistry.byName[qname]; ok {
		panic(fmt.Sprintf(
			"def.Register: duplicate entity name %q (already registered from module %q)",
			qname, existing.EntityModule(),
		))
	}

	globalRegistry.byName[qname] = def
	globalRegistry.defs = append(globalRegistry.defs, def)
}

// All returns a snapshot of all registered EntityDefinitions in registration
// order. The returned slice is a copy — mutations do not affect the registry.
//
// All may be called before or after [Seal]. The registry package calls All
// exactly once during the Compilation Phase.
func All() []EntityDefinition {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	out := make([]EntityDefinition, len(globalRegistry.defs))
	copy(out, globalRegistry.defs)
	return out
}

// Lookup returns the EntityDefinition registered under name, or nil if no
// definition with that name exists.
func Lookup(name string) EntityDefinition {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.byName[name]
}

// Seal marks the registry as read-only. Subsequent calls to [Register] panic.
// Seal is called by the compiler at the start of the Compilation Phase to
// enforce that no definitions are added after compilation begins.
//
// Seal is idempotent — calling it multiple times is safe.
func Seal() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.sealed = true
}

// IsSealed reports whether [Seal] has been called.
func IsSealed() bool {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.sealed
}

// Count returns the number of registered definitions.
func Count() int {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return len(globalRegistry.defs)
}

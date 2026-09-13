package module

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a minimal semantic version (Major.Minor.Patch). It carries no
// pre-release or build-metadata fields — sufficient for module dependency
// resolution without an external dependency.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a version string in "Major.Minor.Patch" form.
// All three components are required and must be non-negative integers.
func ParseVersion(s string) (Version, error) {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("module.ParseVersion: %q: expected Major.Minor.Patch", s)
	}
	parse := func(field, raw string) (int, error) {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("module.ParseVersion: %q: %s must be a non-negative integer", s, field)
		}
		return n, nil
	}
	major, err := parse("Major", parts[0])
	if err != nil {
		return Version{}, err
	}
	minor, err := parse("Minor", parts[1])
	if err != nil {
		return Version{}, err
	}
	patch, err := parse("Patch", parts[2])
	if err != nil {
		return Version{}, err
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// String returns the canonical "Major.Minor.Patch" representation.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Less reports whether v is strictly earlier than other.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// Compatible reports whether other satisfies the constraint expressed by v:
// same Major version and other >= v. This follows semantic versioning
// compatibility semantics.
func (v Version) Compatible(other Version) bool {
	if other.Major != v.Major {
		return false
	}
	return !other.Less(v) // other >= v
}

// Manifest describes an Awo business module (Finance, HR, CRM, etc.).
// A module is registered with [ModuleRegistry.Register] and resolved into a
// startup-safe load order via [ModuleRegistry.Resolve].
type Manifest struct {
	// Name is the stable lowercase module identifier, e.g. "finance".
	// Referenced by other modules in DependsOn and embedded in entity names.
	// Never rename after data is persisted.
	Name string

	// Version is the semantic version of this module release.
	Version Version

	// Label is the human-readable display name, e.g. "Finance".
	Label string

	// Description is a short prose description of what the module provides.
	Description string

	// DependsOn lists modules that must be resolved and started before this
	// module. Each dependency specifies the minimum compatible version.
	DependsOn []Dependency

	// Entities lists the entity names this module registers via
	// definition.Register. Informational only — not validated by ModuleRegistry.
	Entities []string
}

// Dependency declares that a module requires another module at or above a
// minimum version.
type Dependency struct {
	// Module is the name of the required module, matching Manifest.Name.
	Module string

	// Minimum is the lowest version of the dependency that satisfies the
	// requirement. The installed version must have the same Major and be
	// >= Minimum (see Version.Compatible).
	Minimum Version
}

> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Stability Model"
id: gov-stability
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Architecture](documentation-architecture.md)"
  - "[Documentation Standards](documentation-standards.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Stability Model

**Status: Accepted | Stability: Stable**

This document defines the stability levels used in Awo documentation and code, what each level means to consumers, and the rules for changing stability.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Stability Levels

Every document, API, interface, and public function in Awo carries a stability annotation. The four levels are:

### EXPERIMENTAL

- May change or be removed in any release without notice
- Not for use in production code
- Indicates work-in-progress or proof-of-concept
- Documents: may be incomplete or incorrect

### STABLE

- Breaking changes require a deprecation notice in a prior release
- Deprecation notice MUST be visible for at least one minor version before breaking change
- API consumers may rely on STABLE APIs for production use

### FROZEN

- No breaking changes permitted
- Additive-only changes (new optional fields, new endpoints) allowed
- Breaking changes require a new ADR, architecture review, and major version bump
- Documents: content changes require review; structure changes require RFC

### DEPRECATED

- Will be removed in a future version
- The deprecation notice MUST include the replacement API or document
- Deprecated APIs continue to work for at least one major version after deprecation
- Do not write new code using deprecated APIs

---

## 2. Stability in Documentation

Every document declares its stability in the YAML frontmatter:

```yaml
stability: STABLE  # EXPERIMENTAL | STABLE | FROZEN | DEPRECATED
```

The stability level of a document determines the change process required:

| Current Level | Permissible Changes | Process Required |
|---|---|---|
| EXPERIMENTAL | Any change | PR review |
| STABLE | Additive content, clarifications, fixes | PR review |
| STABLE | Content removal, restructure | Architecture discussion |
| FROZEN | Clarifying edits only (no content change) | PR review |
| FROZEN | Any substantive change | RFC + architecture vote |
| FROZEN | Breaking change | New ADR → new document version |

---

## 3. Stability in Code

All exported functions, types, and interfaces in `framework/` packages MUST carry a stability annotation as a Go doc comment:

```go
// GetCompiledEntity returns the compiled entity definition for the given name.
// Stability: STABLE
func (r *Registry) GetCompiledEntity(name string) (*CompiledEntity, bool) { ... }

// RawQuery executes a parameterized SQL query on the entity table.
// Stability: EXPERIMENTAL — API subject to change
func (r *Repository) RawQuery(ctx context.Context, sql string, args ...any) error { ... }
```

The `Stability:` annotation MUST be the last line of the doc comment.

---

## 4. Changing Stability

### EXPERIMENTAL → STABLE

Requires:
- At least one production deployment using the API
- Unit and integration tests covering all documented behaviors
- PR review by at least one framework author

### STABLE → FROZEN

Requires:
- No known design issues
- API has been stable for at least two minor versions
- Architecture team sign-off
- All dependent documents updated to FROZEN

### Any level → DEPRECATED

Requires:
- Replacement API documented and marked STABLE or FROZEN
- Migration guide written
- Deprecation notice in the changelog
- Architecture team sign-off

---

## 5. Stability vs. Bug Fixes

A bug fix is not a breaking change — even in FROZEN APIs. If a FROZEN API has documented incorrect behavior, the incorrect behavior can be fixed without a version bump, but the fix must be documented in the changelog as a behavior correction.

A "breaking change" is defined as: any change that requires consumers of the API or document to update their code or understanding to continue functioning correctly.

---

## Related Documents

- [Documentation Architecture](documentation-architecture.md) — overall documentation governance
- [Documentation Standards](documentation-standards.md) — writing rules
- [Architecture Laws](../02-architecture/laws.md) — LAW-020 (all public APIs carry stability annotations)
- [Glossary](../GLOSSARY.md) — Stability Level, FROZEN, STABLE, EXPERIMENTAL

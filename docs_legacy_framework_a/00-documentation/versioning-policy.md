> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Versioning Policy"
id: gov-versioning
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Stability Model](stability-model.md)"
  - "[Document Lifecycle](document-lifecycle.md)"
  - "[Review Process](review-process.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Versioning Policy

**Status: Accepted | Stability: Stable**

This document specifies how versions are assigned to the framework, how version numbers appear in documentation, and the rules governing breaking changes.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Semantic Versioning

Awo follows Semantic Versioning 2.0.0: `MAJOR.MINOR.PATCH`.

| Component | Increments When |
|---|---|
| `MAJOR` | Breaking changes to FROZEN APIs or Architecture Laws |
| `MINOR` | New STABLE features; EXPERIMENTAL features added or removed |
| `PATCH` | Bug fixes; documentation corrections; no behavior change |

### What Counts as a Breaking Change

A breaking change is any change that requires module authors to modify their code or operators to update their configuration to maintain correct behavior.

Breaking changes include:
- Removing or renaming a STABLE or FROZEN API, function, or type
- Changing the signature of a STABLE or FROZEN interface method
- Changing the behavior of a STABLE or FROZEN API in a way that invalidates existing usage
- Removing a field from a FROZEN response format
- Changing a FROZEN migration that has already been applied

Breaking changes do NOT include:
- Adding optional fields to a STABLE API
- Adding new endpoints
- Fixing incorrect behavior (documented as behavior correction in changelog)
- Changing EXPERIMENTAL APIs (no stability guarantee)

---

## 2. Documentation Versioning

### `since` Field

Every document includes a `since` field indicating the framework version when the document was introduced:

```yaml
since: "1.0"
```

This is set once when the document is first accepted and never changed.

### Version References in Content

When document content references a specific version ("This behavior was introduced in v1.2"), the version MUST use the full format: `v{MAJOR}.{MINOR}`. Patch versions are not referenced in documentation unless the patch specifically changed documented behavior.

### CHANGELOG

A `CHANGELOG.md` in the repository root tracks changes by version. The documentation section of each changelog entry lists:
- New documents added
- Documents promoted to FROZEN
- Documents deprecated
- Content corrections (behavior corrections)

---

## 3. Pre-v1.0 Versioning

Before v1.0, all stability levels are effectively EXPERIMENTAL — the framework may make breaking changes between any two versions. The v1.0 release is the first version where stability guarantees take effect.

The `since: "1.0"` field on all initial documents reflects this: all initial documents are considered part of the v1.0 release.

---

## 4. Version Compatibility

### Module Compatibility

A module declares compatibility via its `ModuleManifest.Version` field. Module version compatibility with the framework version follows these rules:

- Module MAJOR version matches framework MAJOR version (different MAJOR = incompatible)
- Module MINOR version may be lower than framework MINOR version (framework is backward compatible)
- Module is responsible for declaring its minimum framework version requirement

### Documentation and Code Consistency

Documentation MUST accurately describe the code behavior for the version in the `since` field. A document that describes behavior that has not yet been implemented MUST be marked `status: proposed`, not `status: accepted`.

---

## Related Documents

- [Stability Model](stability-model.md) — what stability levels mean for consumers
- [Document Lifecycle](document-lifecycle.md) — how documents move through lifecycle stages
- [Review Process](review-process.md) — approval process for changes by stability level

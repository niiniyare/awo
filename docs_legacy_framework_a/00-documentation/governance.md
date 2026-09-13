> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Governance"
id: gov-governance
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Architecture](documentation-architecture.md)"
  - "[Review Process](review-process.md)"
  - "[Document Lifecycle](document-lifecycle.md)"
  - "[Versioning Policy](versioning-policy.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Governance

**Status: Accepted | Stability: Stable**

This document specifies the governance structure for the Awo framework: who has authority over what, how decisions are made, and how the framework evolves.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Governance Bodies

### Architecture Team

**Composition:** Three to seven framework authors designated as architecture reviewers.

**Authority:**
- Accept or reject ADRs
- Accept or reject RFCs for FROZEN document changes
- Approve promotion of STABLE → FROZEN
- Designate new architecture team members

**Decision process:** Majority vote. Quorum: majority of team members.

### Maintainers

**Composition:** All contributors with merge access to the repository.

**Authority:**
- Merge PRs for EXPERIMENTAL and STABLE document changes (with required approvals)
- Nominate contributors for architecture team membership

**Decision process:** PR approval model (see [Review Process](review-process.md)).

---

## 2. Authority Hierarchy

When decisions conflict, the authority hierarchy resolves them:

```
Architecture Laws (FROZEN)
    ↓
Architecture Invariants (FROZEN)
    ↓
Architecture Team decisions (ADRs, RFC outcomes)
    ↓
FROZEN documents
    ↓
STABLE documents
    ↓
EXPERIMENTAL documents / GUIDE documents
```

Lower-level documents MUST NOT contradict higher-level documents. If they appear to, the lower-level document is incorrect.

---

## 3. Evolution of Architecture Laws

The 20 Architecture Laws are FROZEN. To change them:

1. Author proposes RFC
2. RFC open for 14-day comment period (double the normal 7 days)
3. Architecture team votes — unanimous agreement required (not majority)
4. If approved: new ADR records the decision, Law is amended, Law counter increments
5. CHANGELOG entry documents the change

This process requires unanimous agreement because Architecture Laws are the foundation that all other documents rest on. Ambiguity in a Law propagates to every document that cites it.

---

## 4. Module Author Rights

Module authors have the right to:
- Propose new document entries (as PRs or issues)
- Report documentation errors without requiring full RFC process
- Request clarification on any STABLE or FROZEN document
- Appeal a review rejection to the architecture team

Module authors do NOT have the authority to:
- Merge their own PRs
- Override FROZEN document content
- Bypass the review process for STABLE changes

---

## 5. Breaking Change Moratorium

During the 30 days before a MAJOR version release:
- No new FROZEN promotions (stability must be established before freeze)
- No new ADRs changing Architecture Laws
- Bug fixes and PATCH-level changes continue normally

This moratorium ensures the MAJOR release has a stable foundation.

---

## 6. Security Overrides

Security vulnerabilities in documentation (incorrect guidance that leads to insecure implementations) bypass all governance processes:

- One architecture team member can approve
- Changes made immediately
- Architecture team notified in parallel
- Formal ADR or RFC submitted within 30 days of the fix

Security correctness takes precedence over governance process.

---

## Related Documents

- [Documentation Architecture](documentation-architecture.md) — structural rules
- [Review Process](review-process.md) — detailed approval mechanics
- [Document Lifecycle](document-lifecycle.md) — document state machine
- [Versioning Policy](versioning-policy.md) — version assignment rules
- [Architecture Laws](../02-architecture/laws.md) — the 20 binding laws

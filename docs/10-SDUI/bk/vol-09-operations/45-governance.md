# Chapter 45 — Governance

> **Volume:** IX — Testing, CI/CD & Governance
> **Phase:** 4
> **Audience:** All Engineers, Engineering Leads, Platform Owners
> **Prerequisites:** Chapters 25–31, 41–44

---

## Table of Contents

- [45.1 Governance Model](#451-governance-model)
- [45.2 Code Review Standards](#452-code-review-standards)
- [45.3 Surface Definition Review](#453-surface-definition-review)
- [45.4 API Change Governance](#454-api-change-governance)
- [45.5 Security Review Process](#455-security-review-process)
- [45.6 Dependency Management](#456-dependency-management)
- [45.7 Incident and On-Call Governance](#457-incident-and-on-call-governance)
- [45.8 Data Governance](#458-data-governance)
- [45.9 Platform Roadmap Governance](#459-platform-roadmap-governance)

---

## 45.1 Governance Model

AwoERP uses a lightweight governance model that prioritizes shipping speed while protecting the stability, security, and trust of a multi-tenant platform. Governance is not bureaucracy — it is the set of decisions that are too expensive to undo.

**The three governance categories:**

| Category | Examples | Review Required |
|----------|---------|----------------|
| **Reversible changes** | Bug fixes, UI tweaks, new feature flags, additive API fields | Standard code review (1 approver) |
| **Consequential changes** | New endpoints, schema changes, new tenant features, surface publications | Code review + checklist sign-off |
| **Irreversible changes** | Breaking API changes, security model changes, data deletion, plugin marketplace approval | Architecture review + security review |

The distinction matters. Most work is reversible; treating it as irreversible slows the team down. Irreversible changes deserve proportional scrutiny.

---

## 45.2 Code Review Standards

### 45.2.1 Review Requirements

| Change Type | Approvals Required | Conditions |
|-------------|-------------------|-----------|
| Bug fix | 1 | Must include a test that catches the bug |
| New feature | 1 | CI must pass; tests covering the feature |
| Auth / security code | 2 | One approver must be a security-designated reviewer |
| Database migration | 2 | Migration review checklist (Chapter 44) signed off |
| Dependency addition | 1 | Dependency review checklist (Section 45.6) signed off |

### 45.2.2 Review Checklist

Reviewers check (not exhaustively — use judgment):

- [ ] **Correctness:** Does the code do what it claims to do?
- [ ] **Tests:** Are there tests? Would the tests catch a regression in the logic they're testing?
- [ ] **Error handling:** Are errors handled? Are they wrapped with context?
- [ ] **Tenant isolation:** Does this code operate correctly with RLS? Does it ever read or write without a tenant context?
- [ ] **Authorization:** Does every mutation check permissions? Does every data read go through RLS?
- [ ] **Observability:** Are significant operations logged at INFO? Are errors logged at ERROR?
- [ ] **Performance:** Does this introduce N+1 queries? Does it lock rows unnecessarily?

### 45.2.3 Author Responsibilities

Before requesting review, the author must:
1. Run `make ci-local` (runs lint, unit tests, and integration tests locally)
2. Write a PR description that explains *why*, not just *what*
3. Add the relevant checklist items as a PR checklist (CI auto-populates them based on changed files)
4. Self-review the diff — read every line before requesting review

---

## 45.3 Surface Definition Review

Surface definitions are product artifacts, not just configuration. Publishing a surface definition to production changes what every user with access to that surface sees. The review process reflects this:

### 45.3.1 Review Requirements

| Change | Review Required |
|--------|----------------|
| New surface | 1 engineer + 1 product owner |
| Existing surface update | 1 engineer |
| Accessibility-affecting change | 1 engineer + accessibility review (automated axe-core + manual spot-check) |
| i18n-affecting change | 1 engineer + confirmation that all locales have translations |

### 45.3.2 Pre-Publication Checklist

- [ ] Surface linter passes (`awoerp-lint surfaces/`)
- [ ] Translation keys present in all supported locales
- [ ] Accessibility validation passes (no `a11y_*` lint errors)
- [ ] Role permutation tests pass (Chapter 41.6.2)
- [ ] Surface preview reviewed in staging for each platform (web, mobile)
- [ ] Snapshot tests updated if compilation output changed

### 45.3.3 Publication Flow

Surface definitions are published through the platform admin, not directly to the database. The platform admin surface requires two-factor authentication for publication and logs the publication event to the audit trail with the publisher's identity.

---

## 45.4 API Change Governance

### 45.4.1 Additive Changes (No Governance Required)

- Adding a new optional response field
- Adding a new endpoint
- Adding a new query parameter (optional)
- Adding a new webhook event type

These are published with standard code review. The OpenAPI spec diff is reviewed as part of the PR.

### 45.4.2 Deprecations

When deprecating an endpoint or field:
1. Add to `deprecations.yaml` (Chapter 44.5)
2. Add `X-Deprecated` + `Sunset` headers to the response
3. Notify tenant developers via the changelog and developer portal
4. Set a calendar reminder for the Sunset date

### 45.4.3 Breaking Changes

A breaking change requires:
1. Architecture review meeting with affected team members
2. Migration guide written before the change ships
3. Backwards-compatibility test added (Chapter 42.6)
4. Announcement to tenant developers at least 14 days before the deprecation period begins

Breaking changes are never rushed. If a security issue requires an immediate breaking change, it follows the incident process (Section 45.5), not the normal deprecation window.

---

## 45.5 Security Review Process

### 45.5.1 Automatic Security Review Triggers

A security review is automatically required (CI enforces this) when a PR touches:
- `internal/auth/` — Authentication and JWT handling
- `internal/authz/` — OpenFGA integration and permission checks
- `internal/db/migrations/` — Database schema (can affect RLS)
- Any file containing `RLS`, `BYPASSRLS`, `SET LOCAL app.tenant_id`, `SET LOCAL app.user_id`
- `plugin-sdk/` — Plugin security model
- Any new dependency (see 45.6)

### 45.5.2 Security Review Scope

The security reviewer checks:
- Authorization: Is every action gated by a permission check?
- Tenant isolation: Is RLS context always set? Could any query bypass tenant isolation?
- Input validation: Is user input validated before use?
- Injection: Are all SQL queries parameterized via sqlc? Are all external inputs sanitized?
- Secret management: Are any credentials hardcoded or logged?
- New dependencies: Could the dependency introduce a supply chain risk?

### 45.5.3 Vulnerability Disclosure

Security vulnerabilities are reported to `security@awoerp.dev`. The response SLA:
- **Critical (CVSS ≥ 9.0):** Acknowledge within 24 hours, patch within 72 hours
- **High (CVSS 7.0–8.9):** Acknowledge within 48 hours, patch within 7 days
- **Medium / Low:** Acknowledge within 5 days, patch within 30 days

Vulnerabilities are not disclosed publicly until a patch is available (responsible disclosure).

---

## 45.6 Dependency Management

### 45.6.1 Adding a New Dependency

Every new dependency (Go module, npm package) is reviewed against:

- [ ] **License:** Is the license compatible with commercial use? (MIT, Apache 2.0, BSD — yes. AGPL, GPL without exception — no without legal review.)
- [ ] **Maintenance:** Is the dependency actively maintained? Last commit within 12 months?
- [ ] **Popularity / trust:** Is it widely used in the ecosystem? Would a supply chain attack affect many other projects?
- [ ] **Scope:** Does this package do more than needed? Prefer small, focused packages.
- [ ] **Vulnerability history:** Does it have a history of critical CVEs?

### 45.6.2 Automated Dependency Updates

Dependabot (Go) and Renovate (npm) run weekly and open PRs for dependency updates. Updates are categorized:

| Update Type | Auto-merge? | Conditions |
|-------------|------------|-----------|
| Patch (x.y.Z) | Yes | CI passes |
| Minor (x.Y.z) | No | Reviewed by a human |
| Major (X.y.z) | No | Reviewed + migration guide read |
| Security fix (any) | No | Security reviewer approval |

### 45.6.3 License Compliance Scan

`go-licenses` and `license-checker` (npm) run in CI and output a license report. Any dependency not on the approved license list blocks the build until reviewed.

---

## 45.7 Incident and On-Call Governance

### 45.7.1 On-Call Rotation

The platform has a 24/7 on-call rotation. The on-call engineer:
- Acknowledges P1/P2 PagerDuty alerts within 15 minutes
- Owns the incident until resolved or handed off
- Writes the incident timeline in the incident Slack channel in real time
- Initiates the post-incident review within 48 hours of resolution

### 45.7.2 On-Call Expectations

- On-call engineers are not expected to solve every problem alone. They are expected to triage, escalate, and communicate.
- A P1 incident escalates to the engineering lead after 30 minutes without resolution.
- On-call shifts rotate weekly. No engineer is on-call for more than one week in four.

### 45.7.3 Post-Incident Review (PIR) Governance

PIRs are blameless. The goal is to understand what happened, not to assign fault. PIR documents are:
- Written within 48 hours of the incident
- Reviewed by at least two engineers (not the incident responder)
- Published to the internal engineering wiki
- Action items tracked in the engineering backlog with owners and due dates

Action items from PIRs are prioritized above new feature work during the next sprint.

---

## 45.8 Data Governance

### 45.8.1 Data Classification

| Class | Examples | Access |
|-------|---------|--------|
| Public | Documentation, API schema | Anyone |
| Internal | Logs, metrics, telemetry | AwoERP engineers |
| Confidential | Tenant business data (invoices, employees) | Tenant users only (RLS enforced) |
| Restricted | Encryption keys, credentials, PII | Least-privilege; no logging |

### 45.8.2 Data Retention and Deletion

| Data Type | Retention | Deletion Method |
|-----------|-----------|----------------|
| Tenant business data | While tenant is active | GDPR deletion request: 30-day soft delete, then hard delete |
| Audit logs | 7 years | Cannot be deleted (compliance) |
| Application logs | 90 days | Automatic expiry |
| Telemetry | 90 days | Automatic expiry |
| Backups | 30 days | Automatic rotation |

### 45.8.3 GDPR Deletion Request Process

1. Tenant admin submits deletion request via the platform admin surface
2. Platform queues a Temporal workflow: `TenantDataDeletionWorkflow`
3. 30-day grace period begins (tenant can cancel)
4. After 30 days: all tenant data is hard-deleted, including from backups created after the request
5. Audit log entries for this tenant are anonymized (user IDs replaced with `[deleted]`)
6. Tenant is notified of completion

The deletion is verified by querying every `tenant_id`-scoped table and confirming zero rows.

### 45.8.4 Data Access for Engineers

Production data access by engineers follows the principle of least privilege:
- **Application logs:** Available to all engineers via Grafana
- **Tenant business data:** Requires explicit approval (security lead + engineering lead). Access is time-limited (4 hours), read-only, and fully audited. Every query is logged.
- **Encryption keys:** Never accessible to engineers. Key management is handled by the HSM/KMS; keys are not exportable.

---

## 45.9 Platform Roadmap Governance

### 45.9.1 Roadmap Process

The platform roadmap is managed in six-week planning cycles:

1. **Week 1:** Engineering leads review incoming requests (tenant feedback, PIR action items, tech debt backlog)
2. **Week 2:** Prioritization meeting: capacity vs. demand; produce a ranked list
3. **Weeks 3–8:** Execution sprint
4. **End of cycle:** Demo, retrospective, next cycle planning begins

### 45.9.2 Breaking Change Moratorium

No breaking API changes ship in the four weeks before a major calendar period (end of financial year, peak retail season). The moratorium is declared at the start of each planning cycle.

### 45.9.3 Platform Version Policy

AwoERP follows semantic versioning for its platform version:
- **Major (X.0.0):** Breaking API or SDUI schema change
- **Minor (0.Y.0):** New features, new endpoints, new surface types
- **Patch (0.0.Z):** Bug fixes, security patches, performance improvements

The minimum supported client version is always documented and enforced via the `X-Schema-Version` handshake (Chapter 42.6.2).

---

*End of Chapter 45*

---

*End of Volume IX — Testing, CI/CD & Governance*

**Previous:** [Chapter 44 — Migration](./44-migration.md)
**Next:** [Chapter 46 — Best Practices](../vol-10-reference/46-best-practices.md)

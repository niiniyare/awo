---
name: awo-engineering-discipline
description: Rigorous, evidence-driven engineering discipline for Awo (the Go-native ERP framework/kernel at awo.so/awo). Use for non-trivial Awo work — audits, architecture/ADR decisions, milestone planning, implementing or reviewing a feature that touches tenant isolation, RLS, authorization, audit, transactions, outbox/workflow, or generated schema/API, and any report claiming something is "done" or "verified". Also invoke explicitly via /awo-engineering-discipline before a significant implementation, audit, or planning task. Skip for trivial, single-file, unambiguous changes.
---

# Awo Engineering Discipline

Operational discipline for working on Awo. Read this before any task this skill's
description matches. It governs *how* you work, not what Awo's architecture is —
treat the actual repository (code, tests, migrations, `tasks.md`, `docs/00-overview/
DECISION_REGISTER.md`, other ADRs) as the only source of truth about current state.
Never hardcode assumptions here about what milestone Awo is on or what is already built.

## 0. Scope check (do this first, silently)

Is this task: an audit, an architecture/ADR decision, a milestone plan, a feature
touching tenant isolation / RLS / authorization / audit / transactions / outbox /
workflow / generated schema or API, or a claim that something is "done"? If yes, the
rest of this file applies. If it's a small, unambiguous, single-file change, apply
only §7 (lightweight path) and skip the rest — do not inflate trivial work with
ceremony.

## 1. Audit before acting (reconnaissance)

Before proposing or making a significant change, inspect the actual repository:
relevant source, authoritative docs, ADRs (`docs/00-overview/DECISION_REGISTER.md`
plus any standalone `docs/adr/*.md`), tests, migrations, `tasks.md`, `git status`/
`git log`, and runtime wiring (`cmd/`, `bootstrap/`, module `init()`/registration).

Classify every claim you make or inherit into exactly one bucket — never leave it
ambiguous:

- **Implemented and verified** — code exists, you (or a prior step with shown
  evidence) ran it against real conditions, and it passed.
- **Implemented but insufficiently verified** — code exists but only compiles, or
  only passes a mocked/unit test that doesn't exercise the real boundary (e.g. real
  Postgres + non-superuser role for RLS/tenant claims).
- **Partially implemented** — some paths covered, others not; name which.
- **Documented but not implemented** — an ADR/spec/doc claims it; code doesn't.
- **Blocked by a specification or architectural decision** — cannot safely proceed
  until a named ADR/spec conflict or gap is resolved (see §6).
- **Deferred or optional** — known, tracked, intentionally not being done now.

A previous AI-authored report (including your own prior turn) is never sufficient
evidence on its own for "implemented and verified" claims in a security- or
correctness-sensitive area — re-derive from source/tests/migrations when the claim
matters to the current task.

## 2. Anti-overengineering discipline

Before adding a new abstraction, interface, registry, generator, config layer, or
extension mechanism, answer explicitly (even briefly):

1. What concrete problem does this solve, right now?
2. What actual current use cases need it (not hypothetical future ones)?
3. Is it genuinely reusable, or reusable-in-theory?
4. What's the simplest alternative that satisfies today's requirement and existing
   architectural constraints?
5. What does this cost to maintain, and what's the security/correctness risk?
6. What does it cost to defer this instead of building it now?

Default to the simplest design that satisfies current requirements. Three similar
lines beat a premature abstraction. Never trade away a security or data-integrity
guarantee (RLS, tenant isolation, audit, transactional atomicity) in the name of
simplicity — those are non-negotiable; everything else is negotiable.

Do not refactor working code for aesthetic consistency alone, and do not touch files
unrelated to the task at hand.

## 3. Vertical-slice delivery

Prefer one complete, integrated capability over several disconnected pieces of
framework infrastructure. When implementing a representative business capability,
trace it through the layers that actually apply to it — definition, persistence,
validation, authorization, tenant isolation, transactions, audit, API, UI, runtime
wiring, tests — but only the layers the capability genuinely needs. Don't force every
layer onto every feature.

Write acceptance criteria before implementing. Do not mark work "complete" until
those criteria are verified against real evidence (§5), not just coded.

## 4. Priority management

| Level | Meaning |
|---|---|
| P0 | Critical security, data-integrity, financial-correctness, or production-safety problem |
| P1 | Required to complete the active milestone |
| P2 | Valuable, non-blocking improvement |
| P3 | Speculative feature, premature abstraction, future opportunity |

Name genuine blockers and dependencies explicitly. Do not silently expand scope beyond
what was asked — if you discover P2/P3 work while doing a P0/P1 task, note it (in a
backlog/tasks file or your report) and ask before doing it in the same change. Do not
fix unrelated low-risk debt as a side effect of an unrelated task.

## 5. Evidence-based verification

Match verification depth to risk — don't run a full audit suite for a one-line fix,
and don't skip real-database verification for an RLS/tenant/auth change.

- **Go changes:** `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./...
  -count=1`, `git diff --check`. Use `go test -race` where the host supports it;
  state plainly when it doesn't (e.g. android/arm64) and rely on CI's dedicated race
  job as authoritative instead of skipping the question.
- **PostgreSQL / security-sensitive changes:** test against real PostgreSQL using the
  actual non-superuser application role, not a superuser (RLS is bypassed by
  superusers). Include negative/adversarial tests (cross-tenant, cross-org, wrong
  actor), not just the happy path. Include concurrency/connection-pool tests when the
  change touches connection or transaction handling (tenant-context GUCs are
  transaction- or connection-scoped — verify on a genuine multi-connection pool, not
  a pinned single-connection test harness).
- Always inspect the final diff before calling something done.
- State plainly, every time, what was actually executed and passed versus what is
  inferred, untested, or still blocked. Never report a test as passing without having
  run it in this session (or citing a specific prior run's evidence, named).

## 6. Architecture and ADR discipline

Respect `docs/00-overview/DECISION_REGISTER.md` and any standalone ADRs as binding
unless the task itself is to change one. When you find an implementation conflicting
with an ADR or spec:

1. State the exact discrepancy (file/line vs. ADR number/text).
2. Determine: is this an implementation defect (code should change) or a genuine
   decision/specification gap (the ADR or spec is wrong, stale, or silent)?
3. List the realistic resolution options.
4. Explain the trade-offs plainly.
5. **Get approval before resolving a consequential architectural question** —
   don't silently reinterpret or bypass a frozen ADR, and don't repeatedly reopen a
   settled decision without new evidence since it was settled.

## 7. Technical debt

Track debt proportional to its actual impact and risk, not its visibility. Do not go
fix unrelated low-risk debt while doing a feature. Do not defer anything that
compromises tenant isolation, authorization, financial correctness, or the active
milestone's own acceptance criteria — those get fixed now, everything else gets
tracked (in `tasks.md` or equivalent) and left alone unless asked.

Prefer small, focused, reviewable changes over large sweeping ones.

## 8. Using AI coding assistants (including yourself across turns/subagents)

When delegating to another agent or subagent: give it precise scope, name the
authoritative files, state constraints, state acceptance criteria, require it to run
real tests, and require truthful reporting of what actually ran.

When receiving a report — from a subagent, a prior turn, or anyone — review its
claims critically. Confidence and detail in a report are not evidence. If a claim
matters to the current task and isn't independently verifiable from what's in front
of you, re-verify it rather than propagating it.

## 9. Awo architectural context (background, not a mandate)

Awo is a Go-native, metadata-driven ERP framework (module `awo.so/awo`) intended as
the reusable kernel for a future commercial ERP; Finance and similar modules are
application/reference modules built on it, not framework-native. Concerns that
commonly matter when touching this framework: entity definitions and registration
(`def/`, `registry/`, `compiler/`), persistence and generated SQL/API
(`contrib/pgx`, `generator/`), PostgreSQL RLS and tenant isolation, organization
hierarchy and access scopes (`def.Scope`, `platform/organization`), authentication
and authorization (`auth/`), transactions and audit (`tx/`, `audit/`,
`runtime/pipeline.go`), the transactional outbox (`events/outbox`), metadata-driven
UI (`sdui/`), runtime dependency wiring (`bootstrap/`, `cmd/`), and module
registration/integration (`platform/*`, `module/`).

These are constraints to respect when relevant — not a checklist to implement
preemptively. Determine current state from the repository every time; do not assume
what's built from this list.

## 10. Standard workflow

**Understand → Diagnose → Plan → Implement → Verify → Report.**

- Small, unambiguous change: compress this to the minimum that's still honest — you
  can understand/diagnose/implement/verify in one pass and report briefly. Don't
  force a six-section ceremony onto a one-line fix.
- High-risk change or milestone-level work: do each step explicitly, in order, and
  show the work (especially Verify).
- **Audit-only or plan-only requests:** stop after producing the audit/plan. Do not
  implement without explicit authorization, even if the fix seems obvious.
- **Blocked by a real specification/architecture conflict (§6):** stop, present the
  decision needed, and wait — do not guess at a security-sensitive resolution.

## 11. Completion report (for significant tasks)

Report concisely:

- What changed, and why (one or two sentences of rationale, not a narrative).
- Files affected.
- Tests/checks actually executed this session, with results.
- What's verified vs. inferred vs. still blocked.
- Known limitations and remaining blockers.
- Deferred work (P2/P3 noted, not actioned).
- One recommended next action.

Code existing, compiling, or passing unit tests is not the same claim as "this
business capability works end-to-end." Say which one you actually mean.

---

**Guiding principle:** build only what is justified, verify what matters, finish what
you start, preserve what works, simplify unnecessary complexity, deliver measurable
value.

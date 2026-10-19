# AwoERP Server-Driven UI Platform — Architecture Documentation

> **Version:** 1.0.0 (Phase 1 Complete)
> **Status:** Phase 1 Released · Phase 2 In Progress
> **Maintainer:** AwoERP Platform Architecture Team

---

> **Disposition note (2026-10-19, `tasks.md` R.19, findings register §B.19):** this entire tree is a vision/specification document for a different, larger, unbuilt product ("AwoERP" — its own microservices, native iOS/Android/TUI clients, a React/Zustand/TanStack web stack). It contains zero references to this repository's real `awo.so/awo` packages or ADR numbers anywhere across its 51 chapters — confirmed by repository-wide search, not assumed from its name or directory case. **It is not authoritative for this repository's actual SDUI implementation.** For the real, current architecture, see [`docs/10-sdui/ARCHITECTURE.md`](../10-sdui/ARCHITECTURE.md) (lowercase directory). This tree is retained here as historical/vision material and has not been modified, merged, or deleted — only this notice was added.
>
> A small amount of subject matter here (an accessibility/WCAG mapping in ch.37, a numeric test-pyramid target in ch.41, an API envelope/versioning discussion in ch.25) describes real engineering concerns this repository currently has no normative policy for at all. None of the concrete symbols, endpoints, or stack choices in those chapters apply to this repository — extracting the underlying *ideas* (not the text) into `docs/10-sdui/` remains a separate, unscoped future task.

---

## Documentation Structure

| Volume | Chapters | Phase | Status |
|--------|----------|-------|--------|
| [Vol I — Vision & Philosophy](./vol-01-vision/) | 01–04 | Phase 1 | ✅ Complete |
| [Vol II — DSL & AST](./vol-02-dsl-and-ast/) | 05–08 | Phase 1 | ✅ Complete |
| Vol III — Component System | 09–16 | Phase 2 | 🔄 Planned |
| Vol IV — Rendering Architecture | 17–21 | Phase 2 | 🔄 Planned |
| Vol V — Runtime & State | 22–24 | Phase 3 | 🔄 Planned |
| Vol VI — Platform & API | 25–31 | Phase 3 | 🔄 Planned |
| Vol VII — Reliability & Performance | 32–35 | Phase 3 | 🔄 Planned |
| Vol VIII — Developer Experience | 36–40 | Phase 4 | 🔄 Planned |
| Vol IX — Engineering Excellence | 41–45 | Phase 4 | 🔄 Planned |
| Vol X — Reference & Guidance | 46–51 | Phase 4 | 🔄 Planned |

---

## Quick Start by Role

| Your Role | Start Here |
|-----------|-----------|
| New to the platform (any role) | [Ch 01 — Introduction](./vol-01-vision/01-introduction.md) |
| Backend / Go engineer | [Ch 07 — AST Design](./vol-02-dsl-and-ast/07-ast-design.md) |
| Mobile engineer | [Ch 05 — SDUI Fundamentals](./vol-02-dsl-and-ast/05-sdui-fundamentals.md) |
| Web engineer | [Ch 05 — SDUI Fundamentals](./vol-02-dsl-and-ast/05-sdui-fundamentals.md) |
| Solution architect | [Ch 03 — Architectural Principles](./vol-01-vision/03-architectural-principles.md) |
| ERP implementer | [Ch 04 — System Overview](./vol-01-vision/04-system-overview.md) |

---

## Chapter Index (Phase 1)

### Volume I — Vision & Philosophy
- [Chapter 01 — Introduction to AwoERP UI Platform](./vol-01-vision/01-introduction.md)
- [Chapter 02 — Design Philosophy](./vol-01-vision/02-design-philosophy.md)
- [Chapter 03 — Architectural Principles](./vol-01-vision/03-architectural-principles.md)
- [Chapter 04 — System Overview](./vol-01-vision/04-system-overview.md)

### Volume II — DSL & AST
- [Chapter 05 — Server-Driven UI Fundamentals](./vol-02-dsl-and-ast/05-sdui-fundamentals.md)
- [Chapter 06 — UI DSL Architecture](./vol-02-dsl-and-ast/06-ui-dsl-architecture.md)
- [Chapter 07 — AST Design](./vol-02-dsl-and-ast/07-ast-design.md)
- [Chapter 08 — JSON Compilation Pipeline](./vol-02-dsl-and-ast/08-json-compilation-pipeline.md)

---

*Full TOC available in: [awoerp-sdui-docs-toc.md](../awoerp-sdui-docs-toc.md)*

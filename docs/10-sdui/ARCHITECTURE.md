# Awo SDUI Architecture — Canonical Specification

**Status:** Canonical entry point for Awo's Server-Driven UI (SDUI) architecture, as of 2026-10-19 (`tasks.md` **R.19**, findings register **§B.19**). This document does not restate the detailed field/widget mapping, renderer JSON shape, page-builder authoring guide, or dark-theme CSS contract already specified in this directory's other files — it explains how the layers fit together, which of this directory's other documents is normative for which concern, which claims in those documents and in `docs/10-SDUI/`, `sdui_architecture.md`, and `sdui_implementation.md` (repo root) are implemented versus aspirational, and where the authoritative source code lives for each layer.

**Why this document exists:** before it, "the SDUI architecture" was spread across five files in this directory (each narrowly scoped), a 51-chapter, ~1700-page vision document for a different, unbuilt product at `docs/10-SDUI/` (uppercase — see Disposition, below), and two large root-level planning documents (`sdui_architecture.md`, `sdui_implementation.md`) that are in several ways more accurate about the real package layout than anything in this directory was, yet sat outside it entirely. No single document described the layer responsibilities, dependency direction, or renderer contract as a whole, accurately, in one place. This document is that place.

---

## 1. Purpose and Non-Goals

**Purpose:** Awo's SDUI layer generates a server-driven UI description from entity metadata — field definitions, permissions, lifecycle state, and view configuration — without the frontend re-deriving business rules, authorization, or validation. The browser renders what the backend decided to show; it does not decide what should be shown.

**Non-goals:**
- SDUI is not a general-purpose UI framework or design system. It generates *data-driven CRUD and dashboard surfaces* from entity metadata — list, create, edit, detail, and dashboard views. It is not a page builder for arbitrary marketing pages or free-form layouts.
- SDUI is not AMIS. AMIS (`sdui/amis`) is the one renderer implementation that exists today. The framework-level contract (`sdui/widget`, `sdui/renderer`) is designed to support other renderers (see §13) but none exist yet — this document distinguishes the two deliberately throughout.
- SDUI does not replace server-side authorization or lifecycle enforcement. It *consumes* the results of both (via `auth.ViewerContext` and entity hooks) to decide what to generate; it is not where either is defined or where either could be weakened. See §7.
- Awo is a general-purpose, metadata-driven entity framework; `examples/expense` is a consumer of SDUI, not a source of SDUI architecture. No `sdui/*` package imports anything under `examples/`, and none should.

---

## 2. Architectural Principles and Invariants

These are drawn from ADR-006 (`docs/00-overview/DECISION_REGISTER.md`) and the normative requirements already stated in `sdui/widget/node.go`'s own doc comment — restated here because they govern every layer, not just the widget IR:

1. **The generator never emits renderer-specific constructs.** No AMIS types, CSS classes, or JavaScript strings originate in `sdui/generator`. Confirmed: `sdui/generator/generator.go` imports `sdui/plugins`, `sdui/sduictx`, `sdui/widget` only — never `sdui/amis` or `sdui/renderer` (verified via `rg` this pass, zero matches).
2. **Permission gating is structural absence, not a hidden flag.** A field, section, tab, panel, or action the viewer cannot see is omitted from the tree entirely — never included with `Hidden: true`. `Hidden` is reserved for API-accessible-but-UI-suppressed bookkeeping fields (`sdui/widget/node.go` doc comment). This is a real, enforced invariant, not aspirational: `sdui/generator/generator.go` calls `ctx.Viewer.HasPermission(...)` and conditionally omits nodes at the field (line ~1234-1237), section (~855), tab (~809), panel (~740-743), and action/button (create/update/delete, ~1013-1204) levels — verified this pass by direct inspection, not inferred from documentation.
3. **The widget tree is a frozen, renderer-neutral intermediate representation (IR).** `sdui/widget.Node`, `NodeKind`, `DataSource`, and `ActionNode` are public contracts frozen at v1.0 (ADR-006). `sdui/widget` has zero internal dependencies — confirmed via `rg "awo.so/awo" sdui/widget/*.go`: the only hits are the package's own external test files importing itself.
4. **Server-side validation and lifecycle enforcement are always authoritative; client-side rules are a convenience only.** `widget.Node.Validation`/`Required` exist for immediate UI feedback; `sdui/widget/node.go`'s doc comment states server-side validation "is always authoritative" independent of them. Lifecycle transitions (e.g. `examples/expense`'s `StatusTransitionGuard`) are enforced as entity `BeforeUpdate` hooks — backend code the generator and renderer never see or can bypass.
5. **A renderer is selected, not hardcoded.** `sdui/engine.Options.Renderers map[string]renderer.Renderer` is a real registry keyed by renderer ID, with a `DefaultRendererID` fallback and per-request `RendererID` selection (`sdui/engine/engine.go`). `sdui/amis` is the only entry in that map today, by configuration at bootstrap — not by any `sdui/engine` or `sdui/generator` code path hardcoding it.

---

## 3. Layer and Package Responsibilities

Each row names the real package, its actual responsibility (verified this pass via its own doc comment plus direct code inspection — not inferred from the package name), and whether it is wired into the running application (`cmd/awo`, the binary the Makefile builds).

| Package | Responsibility | Wired in `cmd/awo`? |
|---|---|---|
| `def` | Entity/field/action/hook/permission metadata types. The source of business metadata. | Yes — foundational |
| `registry` | Accumulates `def.Register` calls, validates, seals. | Yes (`bootstrap/bootstrap.go`) |
| `compiler` | `EntityDefinition` → `*compiler.EntitySchema` (dependency graph, fingerprint, capability grants). | Yes |
| `sdui/sduictx` | `GeneratorContext` — the immutable, pass-by-value input that drives every generation decision (view mode, viewer, tenant, locale). No setters, no pointer receivers — plugins physically cannot mutate it. | Yes |
| `sdui/generator` | `CompiledSchema` + `GeneratorContext` → `widget.Node` tree. One `EntityGenerator` for all view modes (list/create/edit/detail/dashboard) — view mode is a `GeneratorContext` field, not a separate type. 5 real pipeline stages (see §5); permission gating happens here. | Yes |
| `sdui/widget` | The frozen IR: `Node`, `NodeKind` (39 constants, recounted precisely this pass — a prior session's "35" was itself imprecise), `DataSource`, `ActionNode`, `LayoutHint`, `ExpressionRef`. Zero internal dependencies. | Yes — imported by nearly every other `sdui/*` package |
| `sdui/expression` | A portable expression DSL — `ExpressionRef` values the generator emits and each renderer evaluates in its own target language (never a raw AMIS JS string from the generator). | Yes |
| `sdui/validation` | Hard pre-render gate: nil nodes, unregistered `NodeKind`, missing required `Name`/`DataSource`, `NodeTabPane`/`NodeSection` nesting rules, a direct self-reference cycle guard, `ExpressionRef` structural validity. **Gap confirmed this pass:** its own doc comment also claims "Plugin ValidationFuncs are invoked and their issues collected" (`sdui/validation/validation.go:16`) — no such invocation exists anywhere in the file or package; see §8 and §12. | Yes |
| `sdui/layout` | Computes column spans/rows from a `widget.Node` tree. Its own doc comment states it explicitly: "no AMIS-specific or renderer-specific behaviour." | Yes |
| `sdui/registry` (NodeKind catalog — distinct from the top-level `registry` package) | Builtin `NodeKind` → widget-definition catalog, consulted by `sdui/validation` to check a kind is registered. | Yes, but narrow — `sdui/validation` is its only consumer |
| `sdui/cache` | The three-level SDUI cache (own doc comment, `sdui/cache/cache.go:1-16`): L1 (in-process `CompiledSchema` cache, owned by `compiler`, not this package), L2 (Redis widget-tree cache, 5 min TTL), L3 (Redis rendered-output cache, 10 min TTL). Real Redis, not a stub — confirmed by package inspection. | Yes |
| `sdui/plugins` | The extension pipeline. 9 documented `ExtensionPoint` constants; only 4 are ever actually invoked by any real pipeline stage, and only 3 `Register*` functions exist to register against them. **Zero production call sites register any plugin anywhere in the repository** — confirmed via `rg "plugins\.Register"` excluding tests and the package itself; the only non-test hit anywhere is prose in `sdui_implementation.md`. See §8 for the precise breakdown. | Technically wired (the generator checks for registered plugins at the points it does invoke) but inert in production today |
| `sdui/renderer` | Defines the `Renderer` interface (`ID`, `Version`, `Render(root, RendererContext) (RenderedOutput, error)`) and the renderer registry (`Registry.Register`). `RenderedOutput` is a typed union, not `map[string]any` — deliberately, to support a future binary (PDF) or typed-struct (Flutter) renderer alongside JSON (own package doc comment). Also carries locale/message formatting (`locale.go`, `messages.go`). | Yes |
| `sdui/amis` | The one concrete `Renderer` implementation today: `widget.Node` → amis JSON (`map[string]any`). ADR-006-bound: must not accept raw field/entity definitions. | Yes |
| `sdui/adapt` | `*compiler.EntitySchema` → `generator.EntitySchema` bridge, specifically to avoid an import cycle between `compiler` and `sdui/generator`. Single real call site: `api/sdui/handler.go`. | Yes |
| `sdui/engine` | Top-level orchestrator: resolve renderer by ID → L3 cache → L2 cache → `Generator.Generate` → store → `validation` (hard gate) → `layout` → `Renderer.Render`. The real dispatch-by-`RendererID` mechanism (§13). | Yes — the real call path from HTTP down |
| `sdui/conformance` | Test-only package verifying cross-package invariants: every `NodeKind` has an AMIS renderer case, generator/renderer output determinism, locale/RTL/decimal-separator behavior, theme config for `dark` and `antd` themes. Not production code — it *is* part of the testing strategy (§11). | N/A (test-only) |
| `api/sdui` | HTTP layer: extracts params, adapts `auth.ViewerContext` → `sduictx.ViewerContext`, builds `GeneratorContext`, calls `sdui/engine`, serializes, sets ETag/Cache-Control. | Yes |
| `api/response` | The canonical HTTP envelope (`{data,meta}` / `{error}`) — not SDUI-specific, but what `api/sdui` and every other handler return. Ratified unchanged by this document; see ADR-026 for the frontend side of this contract. | Yes |
| `web/dist/index.html` | Browser shell: transport (`fetcher`), SPA navigation (`jumpTo`), AMIS-compatibility bridging (`adaptResponse`). Ratified as the maintained frontend source by **ADR-026** — unchanged by this document. | Yes (`web/web.go` `//go:embed`) |

---

## 4. Dependency Direction

```
def  ──────────────────────────────────────────────────────────┐
  │                                                              │
  ▼                                                              │
registry → compiler → sdui/adapt → sdui/generator ──imports──→  sdui/sduictx, sdui/widget, sdui/plugins, sdui/expression
                                        │
                                        ▼
                                  widget.Node tree
                                        │
                                        ▼
                              sdui/validation (+ sdui/registry)
                                        │
                                        ▼
                                  sdui/layout
                                        │
                                        ▼
                     sdui/renderer (interface) ◄── registered at bootstrap ── sdui/amis (implementation)
                                        │
                                        ▼
                                 RenderedOutput (amis JSON today)
                                        │
                                        ▼
                               api/sdui → api/response
                                        │
                                        ▼
                         web/dist/index.html → amis.embed() (browser)
```

Allowed dependency direction, stated explicitly (confirmed by import inspection, not assumed from the diagram's shape):

- `sdui/widget` depends on nothing in `awo.so/awo` except the standard library. Everything else may depend on it.
- `sdui/generator`, `sdui/validation`, `sdui/layout`, `sdui/plugins` depend on `sdui/widget`/`sdui/sduictx` and never on `sdui/amis` or `sdui/renderer`.
- `sdui/renderer` defines the contract; `sdui/amis` is the only package that implements it. `sdui/renderer` does not import `sdui/amis` (the dependency points the other way: `sdui/amis` implements `sdui/renderer`'s interface, it does not import it as a concrete type beyond that).
- `sdui/engine` is the only package that imports both a `Generator` and a map of `Renderer`s — it is the composition point, not `sdui/generator` or `sdui/amis` directly depending on each other.
- Nothing under `sdui/` imports `examples/*`. `examples/expense` imports `def` and the framework; the dependency arrow points one way.

---

## 5. Metadata-to-Renderer Data Flow

Lifted from `sdui/generator/generator.go`'s own doc comment (5 stages) plus the surrounding engine orchestration (verified, not paraphrased from memory):

1. **Pre-generation plugin transforms** — documented as stage 1 of the generator pipeline (`ExtPreGeneration`). **Not currently invoked anywhere** — see §8/§12; this stage is aspirational in the current implementation.
2. **Permission gating** — fields/sections/tabs/panels/actions without viewer permission are omitted (§2.2).
3. **View mode selection** — which fields appear in list vs. form vs. detail, driven by `GeneratorContext.ViewMode` (`sdui/sduictx`).
4. **Node construction** — the `widget.Node` tree is built from field definitions.
5. **Post-generation plugin transforms** (`ExtPostGeneration`) — this stage *is* real: `generator.go:448` calls `g.pipeline.RunTreeTransforms(plugins.ExtPostGeneration, root, ctx)`.
6. Root node returned to `sdui/engine`, which runs `sdui/validation` (hard gate — abort on any Fatal issue), then `sdui/layout`, then dispatches to the resolved `Renderer.Render`.
7. `api/sdui` serializes the renderer's output inside `api/response`'s envelope and returns it over HTTP.
8. The browser shell (`web/dist/index.html`) bridges the envelope (`adaptResponse`) and calls `amis.embed()` with the resulting schema.
9. A user mutation (create/edit/delete/action) goes back through `api/response` → the entity's own hooks (e.g. `StatusTransitionGuard`) enforce lifecycle rules server-side, independent of anything the generator or renderer decided.
10. A successful mutation triggers navigation/refresh in the browser shell (`jumpTo`, `recordIdFromPath` — ADR-026's domain, unchanged here).

---

## 6. Renderer-Neutral IR and Renderer Contract

**Is `widget.Node` sufficiently renderer-neutral?** Yes, confirmed by direct field-by-field inspection this pass (`sdui/widget/node.go`), not merely by its own doc comment's claim:

- Every field is named and typed generically: `Label`, `Name`, `Required`, `ReadOnly`, `Hidden`, `Options []StaticOption`, `DataSource`, `Actions`, `MinValue`/`MaxValue`, `AcceptedTypes`, `MultiSelect`, `Icon` (explicitly documented as a *semantic* icon name — `"user"`, `"calendar"` — that "Renderers map ... to their icon library (e.g. AMIS: fa-* prefix, Flutter: Material icons)").
- `Props map[string]any` is the single, explicit, documented escape hatch for anything renderer-specific — "use sparingly — prefer typed fields where possible." This is the correct design for an IR: an explicit, narrow escape valve, not an implicit one.
- The only AMIS references found anywhere in `node.go` are inside doc comments, used as *illustrative examples* of how a renderer might interpret a generic field (e.g. "`NodeSection` ... amis fieldSet or group"; "Renderers convert these to native constraints (e.g., AMIS min/max)"). None of these leak into a type, a required shape, or a hardcoded string the generator emits. **Classification: harmless implementation detail, not a confirmed architectural violation.**
- **Confirmed exception, found during this pass's consistency check, not previously documented here:** the renderer-neutrality guarantee above applies to the *auto-generation* path (`sdui/generator` → `widget.Node` → `Renderer.Render`) only. A **custom `def.PageBuilder`** (declared via `EntityDefinition.PageBuilders`, wired through `api/sdui/handler.go:378-396`) bypasses `widget.Node` and the renderer entirely: it returns `map[string]any` directly, and the handler serves that result as JSON verbatim ("no generator, no cache" — the handler's own comment). `def.PageBuilder`'s own doc comment (`def/page.go:29-33`) confirms this is deliberate, not accidental: it says the function "produces an **amis JSON schema**" that "must conform to the **pinned amis SDK version in `web/sdk/`**." This is a real, live exception to §2's principle #1 for any entity that declares a custom page builder — see §8's disposition note and the open decision recorded in `tasks.md` **R.22**. `def/page.go:33`'s own doc comment additionally repeats the same stale "pinned to `web/sdk/`" claim ADR-026 already corrected in `AMIS_RENDERER_SPEC.md` — found in Go source this time, not a doc, and **not edited here** (out of this mission's explicit scope: no component/source-code changes).

**The renderer contract** (`sdui/renderer.Renderer`): `ID() string`, `Version() string`, `Render(root *widget.Node, ctx RendererContext) (RenderedOutput, error)`. `RendererContext` deliberately carries renderer-interpreted hints (`Theme`, `GridSystem`) that mean nothing to the generator and are constructed by `api/sdui`, never passed to `Generator`. `RenderedOutput` is a typed union specifically so a non-JSON renderer (PDF, a typed Flutter struct) can exist without reshaping the contract — this is already in the real interface today, not a future plan (`sdui/renderer/renderer.go`'s own doc comment names "amis", "flutter", "pdf" as the three illustrative renderer IDs the design anticipates).

---

## 7. Authorization, Validation, and Lifecycle Boundaries

| Concern | Where it is decided | Where the UI reflects it | Can the browser override it? |
|---|---|---|---|
| Field/section/tab/panel/action visibility | `sdui/generator`, via `auth.ViewerContext.HasPermission` | Absent node (never rendered) | No — never generated in the first place |
| Field required-ness, basic shape validation | `def.FieldDef`, re-expressed as `widget.Node.Required`/`Validation` | Immediate client-side feedback (AMIS form validation) | No — server-side validation is authoritative regardless of what the client sent |
| Entity lifecycle transitions (e.g. draft→submitted→approved) | Entity `BeforeUpdate` hooks (e.g. `examples/expense`'s `StatusTransitionGuard`) — pure backend code | Not reflected in the generated schema at all today (the `status` field has no `disabled` flag even when a transition would be rejected — confirmed in a prior session's investigation, findings register §B.16) | No — the backend hook runs regardless of what the UI allowed the user to attempt; a rejected transition returns a real error, which the browser must surface (ADR-026/§B.16's domain) |
| HTTP-level response shape | `api/response` | Bridged by `adaptResponse` in the browser shell | No — `api/response`'s envelope is the single source of truth; the frontend does not invent its own |

The browser is not the enforcement layer for any of these — it is a thin reflection of decisions already made server-side, plus a transport/compatibility bridge. This is the current, real state, not merely a target: every enforcement point above is backend code with no UI-side equivalent that could substitute for it.

---

## 8. Extension and Plugin Strategy

**Documented:** `sdui/plugins/plugins.go`'s own doc comment claims nine extension points (`ExtPreGeneration`, `ExtFieldNodeOverride`, `ExtPostGeneration`, `ExtPreRender`, `ExtCustomNodeRenderer`, `ExtPostRender`, `ExtDashboardPanel`, `ExtValidation`, `ExtLayoutTransform`), each with priority-ordered registration, a Fatal/Recoverable error model, and explicit forbidden operations (no mutating `GeneratorContext` — physically prevented by its value-type design; no bypassing permission gates; no injecting renderer-specific constructs into the tree; no touching fingerprints).

**Actually implemented, verified this pass by direct inspection (not restating the prior architecture review's slightly less precise "3 vs 9" summary):**

| Extension point | Registration function exists? | Actually invoked by any pipeline stage? |
|---|---|---|
| `ExtPostGeneration` | Yes (`RegisterTreeTransform`) | **Yes** — `sdui/generator/generator.go:448` |
| `ExtPreRender` | Yes (`RegisterTreeTransform`, same function, different `point` argument) | No — zero invocations found anywhere outside `sdui/plugins` itself |
| `ExtFieldNodeOverride` | Yes (`RegisterFieldNodeOverride`) | No — zero invocations found |
| `ExtValidation` | Yes (`RegisterValidation`) | No — `sdui/validation/validation.go`'s own doc comment claims this runs ("Plugin ValidationFuncs are invoked and their issues collected"); no such call exists anywhere in that file or package |
| `ExtPreGeneration`, `ExtCustomNodeRenderer`, `ExtPostRender`, `ExtDashboardPanel`, `ExtLayoutTransform` | No registration function exists for any of these | No |

**Production usage:** zero. `rg "plugins\.Register"` across the whole repository, excluding `sdui/plugins` itself and test files, returns exactly one hit — prose in `sdui_implementation.md`, not code.

**Classification (per this mission's required categories):**
- The 4-of-9 gap between documented and registerable extension points: **documented but not implemented** — not a defect to fix via documentation alone; a real follow-up decision (implement the remaining 5, or correct the doc comment's count) is needed. Tracked as `tasks.md` **R.20**.
- `ExtValidation`'s claimed-but-absent invocation in `sdui/validation/validation.go:16`: **confirmed code-level defect** (a doc comment asserting behavior the code does not have) — also tracked as `tasks.md` **R.20**, not fixed here (this mission is documentation-only; fixing it means either wiring the invocation or correcting the comment, both implementation-adjacent decisions out of this mission's scope).
- The zero-production-use fact itself is **not** a defect — per the engineering discipline skill's anti-overengineering guidance, an unused extension point that costs nothing to leave alone is correctly left alone until a real use case appears. Expanding `sdui/plugins` further without one would be the actual overengineering risk.
- **Related, separately scoped exception — not a plugin, but a similar escape hatch:** `def.PageBuilder` (§6's exception note) is a *different* mechanism that also bypasses the standard pipeline, by returning raw amis JSON instead of a `widget.Node`. Unlike the plugin gap (which is dormant/unused), this one is live and real custom builders may already depend on it — a real architectural decision, not a documentation error. Recorded as an **unresolved architectural decision**, not silently ratified either way: `tasks.md` **R.22**.
- **Corroborating evidence, found this pass, not previously cited:** `sdui/README.md` (a 919-line developer guide living in the source tree, not `docs/10-sdui/`, and out of this mission's scope to correct) independently lists `ExtPreGeneration` as a live hook ("Before `Generate()`") and separately states "Two extension points" in its own Plugins section — a third, different count from both the real 9-declared/4-invocable total and `sdui_implementation.md`'s "nine." This corroborates, rather than contradicts, R.20: the plugin system's documentation is scattered and mutually inconsistent across at least three sources, independent of this document.

---

## 9. API and Frontend Responsibilities

Settled by **ADR-026** and unchanged by this document:

- **Backend SDUI generation** (`sdui/generator` → `sdui/engine`): decides what to show, in what shape, gated by permission and view mode.
- **The API endpoint and response envelope** (`api/sdui`, `api/response`): transports the decision as `{data,meta}`/`{error}` — never a renderer-specific shape.
- **The browser shell** (`web/dist/index.html`, ratified as the maintained source by ADR-026): transport, SPA navigation, and AMIS-compatibility bridging only. It does not generate schema, does not decide permissions, and does not re-implement lifecycle rules.
- **The AMIS renderer/runtime** (`sdui/amis` backend + `amis@6.7.0` loaded client-side, ratified by ADR-026): converts the IR to amis JSON server-side; interprets that JSON client-side. The *frontend* half of this (which SDK version, which file is authoritative) is ADR-026's domain and is not reopened here, per this mission's explicit instruction.

---

## 10. Caching and Observability Ownership

Caching is owned entirely by `sdui/cache` (L2/L3) and `compiler` (L1) — not by `sdui/generator`, `sdui/amis`, or any API handler reimplementing cache-key logic. Cache keys embed `{entity, view, renderer_id, renderer_ver, locale, tenant_id_hash, schema_fp, perm_fp}` (`sdui/cache/cache.go`), meaning a schema or permission change, or a renderer version bump, invalidates automatically — no manual cache-busting code exists or is needed elsewhere. `sdui/renderer.Renderer.Version()` feeds directly into this key; `sdui/amis`'s `RendererVersion = "1.1.0"` (its own schema-format version, not an AMIS SDK version — do not confuse the two) is what changes there, not the AMIS SDK build ADR-026 governs.

Observability for the SDUI pipeline specifically was not deeply traced this pass (out of this mission's evidence-gathering scope — general API/request logging exists per prior sessions' work, e.g. findings register §B.12's access-log fix, but no SDUI-pipeline-specific metrics/tracing were found or claimed here). **Labeled explicitly: unresolved / not investigated this pass**, not asserted either way.

---

## 11. Testing and Conformance Strategy

- **Unit tests** per package (`sdui/*/*_test.go`) — the normal Go pattern, present for `widget`, `generator`, `validation`, `layout`, `amis`, `plugins`, `adapt`, `registry`, `engine` (confirmed present in a prior session's package-by-package trace; not re-verified file-by-file this pass).
- **`sdui/conformance`** is the cross-package invariant layer: every `NodeKind` has an AMIS renderer case (`TestAMISRendererCoversAllNodeKinds`), generator and renderer output determinism (`TestGeneratorDeterminism`, `TestRendererDeterminism`), locale/RTL/decimal-separator/language-fallback behavior, and dark/antd theme config. This is real test code, confirmed present this pass, not aspirational.
- **What is not covered by any committed test, confirmed by a prior session's dedicated investigation:** no committed test exercises a generated schema through the actual amis SDK (all such evidence is uncommitted jsdom scratchpad work from earlier sessions — see findings register §B.15/§B.16); `api/handler/crud.go` has no dedicated test file. Neither gap is new to this mission; both remain open, tracked under their original numbers (not duplicated here).

---

## 12. Current Implementation Status and Known Gaps

| Claim | Status | Evidence |
|---|---|---|
| Generator never emits renderer-specific constructs | **Implemented and verified** | Import check, this pass |
| Permission gating is structural absence | **Implemented and verified** | Direct code inspection, this pass (§2.2) |
| `widget.Node`/`NodeKind`/`DataSource`/`ActionNode` frozen IR, zero internal deps | **Implemented and verified** | `rg` import check, this pass |
| Renderer selected via registry, not hardcoded | **Implemented and verified** | `sdui/engine.Options.Renderers`, this pass |
| 3-level cache with real Redis | **Implemented and verified** | Package doc + structure, this pass |
| 9 documented plugin extension points | **Partially implemented** — 4 of 9 are ever invoked; 0 are used in production | This pass, precise count (§8) |
| `ExtValidation` plugin hook | **Documented but not implemented** | `sdui/validation/validation.go:16` vs. actual code, this pass |
| `ExtPreGeneration` pipeline stage | **Documented but not implemented** | Generator doc comment vs. `rg` for its invocation, this pass |
| A second renderer (Flutter/PDF) | **Not implemented; the contract anticipates it** | `sdui/renderer` doc comment names them as illustrative, not built |
| SDUI-pipeline-specific observability | **Unresolved — not investigated this pass** | — |
| Frontend source of truth / AMIS version | **Decided (ADR-026)** — out of this mission's scope to reopen | — |
| `def.PageBuilder` custom overrides preserve renderer-neutrality | **Confirmed architectural exception — they do not.** `def.PageBuilder` returns raw amis JSON directly (`def/page.go:29-33`'s own doc comment), bypassing `widget.Node` and the renderer entirely. Unresolved: whether this is an intentional, accepted escape hatch or a gap to close. | `def/page.go:40`, `api/sdui/handler.go:378-396`, this pass — see `tasks.md` **R.22** |

---

## 13. Adding a Renderer Without Coupling Framework Metadata to It

Concretely, from the real interfaces (not a hypothetical):

1. Implement `sdui/renderer.Renderer`: `ID() string`, `Version() string`, `Render(root *widget.Node, ctx RendererContext) (RenderedOutput, error)`.
2. Register it in the map passed to `sdui/engine.Options.Renderers[your_id] = yourRenderer{}` at bootstrap (wherever `sdui/amis.New()` is registered today — `cmd/awo/serve_impl.go`).
3. Select it per-request via `GeneratorContext`/`RendererID`, or set it as `DefaultRendererID`.

**Nothing else changes.** `def/`, `compiler/`, `sdui/generator/`, `sdui/widget/`, and every entity definition are untouched — none of them know a renderer named `"amis"` exists; they only ever see `*widget.Node`. This is the real, current state of the code, not a target to build toward. The only genuine risk is `Props map[string]any` escape-hatch usage drifting toward one renderer's assumptions over time — a code-review discipline concern, not an architectural gap, since the type system cannot prevent it by design (that is the point of an explicit, narrow escape hatch rather than no escape hatch at all).

---

## 14. Related ADRs and Implementation Documents

- **ADR-006** (`docs/00-overview/DECISION_REGISTER.md`) — the WidgetTree IR requirement this entire document elaborates on.
- **ADR-026** (`docs/00-overview/DECISION_REGISTER.md`) — frontend source of truth and AMIS runtime version. Not reopened by this document.
- `AMIS_RENDERER_SPEC.md` (this directory) — the detailed amis JSON output contract, caching key format, and SDK version note (corrected per ADR-026). Its `Renderer` interface code sample (§2) was corrected this pass: it showed the stale `Render(root *widget.Node) (map[string]any, error)` attributed to package `awo.so/awo/sdui/amis`; the real interface is `Render(root *widget.Node, ctx RendererContext) (RenderedOutput, error)`, defined in `sdui/renderer` and implemented by `sdui/amis` — confirmed via direct inspection this pass, not previously caught by this document's own earlier version.
- `WIDGET_TREE_SPEC.md` (this directory) — the detailed `widget.Node`/`NodeKind` reference. Its documented `NodeKind` list (recounted precisely this pass: 17 entries, not the previously-stated 15) is stale against the real 39 constants (not 35) in `sdui/widget/node.go` — the count itself was corrected in both documents this pass, but `WIDGET_TREE_SPEC.md`'s table was not expanded to list the other 22 real constants (`NodeMoney`, `NodeLookup`, `NodeTreeSelect`, `NodeMultiSelect`, `NodeRichText`, `NodeGrid`, `NodeTabPane`, and 15 more) — doing so is content authorship, not a factual correction, and is left as an open, separately-scoped task.
- `SDUI_FIELD_WIDGET_MAP.md` (this directory) — `FieldType` → `NodeKind` → amis-type mapping, including the `Immutable: true → disabled: true` edit-view rule. Three mapping cells were corrected this pass against the real `fieldTypeToNodeKind` switch in `sdui/generator/generator.go` (`currency`→`NodeMoney`, not `NodeNumber`; `multi_select`→`NodeMultiSelect`, not `NodeSelect`+`multiple:true`; `link`→`NodeLookup` when a `DataSource` is configured, `NodeSelect` otherwise — not unconditionally `NodeSelect`). The table remains incomplete against the full switch statement (`rich_text`, `tree_link`, `duration`, `color`, `signature`, file/image/attach are real `FieldType`s with real mappings not listed) and does not distinguish the separate, simpler mapping `fieldTypeToColumnNodeKind` uses for list columns — both noted as open, separately-scoped completions, not performed here.
- `PAGE_BUILDER_GUIDE.md` (this directory) — how to write a custom `PageBuilder`, including the permission-gating pattern (§2.2) in authoring terms. Its §3 function signature was corrected this pass: it showed an imagined `func(ctx, viewer auth.ViewerContext, schema *compiler.EntitySchema) (*widget.Node, error)`; the real `def.PageBuilder` (`def/page.go:40`) is `func(ctx context.Context, pctx def.PageContext) (map[string]any, error)` — see §6's exception note and §8's `tasks.md` **R.22** for why this matters architecturally, not just syntactically.
- `sdui/README.md` (package source tree, not this directory — out of this mission's scope to correct, noted for completeness) — a 919-line developer guide covering nearly the same ground as this document. Spot-checked, not fully audited, this pass: its `Renderer` interface sample is correct (matches real code); its plugin extension-point description is not (see §8).
- `DARK_THEME.md` (this directory) — dark-mode CSS token contract.
- `docs/10-SDUI/` (uppercase), `sdui_architecture.md`, `sdui_implementation.md` (repo root) — see Disposition, below.

---

## Disposition of Other SDUI-Related Documents

**`docs/10-SDUI/` (uppercase, 51 chapters + README):** Not authoritative for this repository's implementation. Confirmed this pass (re-verifying, not assuming, a prior session's finding still holds after `bk/` and `copy/` were independently removed from the tree): it describes a different, unbuilt product — its own microservices, native iOS/Android/TUI clients, a different web stack (React 18, Zustand, TanStack Query), invented API paths — with zero real `awo.so/awo` package references or real ADR numbers anywhere in the tree. It is retained as historical/vision material, not deleted (per this mission's explicit instruction), with a disposition pointer added to its own `Readme.md` directing readers here. A small number of its concerns (an accessibility/WCAG mapping, a numeric test-pyramid target) describe real engineering topics this repository has no normative policy for at all — flagged as salvageable *subject matter*, not salvageable *text*, since every concrete symbol in them is invented for the other product. Extracting them, if ever done, is a separate, explicitly-scoped future task, not performed here.

**`sdui_architecture.md` / `sdui_implementation.md` (repo root):** More technically accurate about the real package layout than their "historical blueprint" framing in `tasks.md`'s existing (pre-this-mission) note suggested. Confirmed this pass: their prescribed fixes (introducing `NodeTabPane` distinct from `NodeSection`; eliminating a `setAllReadOnly` post-processing hack) are already implemented in the real code exactly as they recommended. Their package inventory (`sdui/dashboard`, `sdui/cache` with real Redis, `sdui/plugins`, `sdui/expression`, `sdui/registry` with `Seal()`, `sdui/engine`) matches reality closely. They are not moved or deleted in this pass (per this mission's explicit instruction not to delete retired assets or perform broad reorganization); each has gained a short header note pointing to this document as the current canonical architecture reference, with their own content retained as historical/supplementary detail — some of it (e.g. the plugin pipeline's documented nine extension points, matched against §8's real invocation count) is still the most detailed available description of a mechanism this document only summarizes.

**This document (`docs/10-sdui/ARCHITECTURE.md`):** Is the canonical entry point. It does not supersede the other four files in `docs/10-sdui/` — they remain normative for their own narrow scope (amis JSON shape, widget/field reference, page-builder authoring, dark theme) — it ties them together and corrects the claims in this directory, `docs/10-SDUI/`, and the two root documents that conflicted with the real, verified implementation.
